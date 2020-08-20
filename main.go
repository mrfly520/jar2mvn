package main

import (
	"context"
	"crypto/md5"
	"flag"
	"fmt"
	"github.com/PuerkitoBio/goquery"
	"github.com/chromedp/cdproto/cdp"
	"github.com/chromedp/cdproto/network"
	"github.com/chromedp/chromedp"
	"github.com/gnewton/jargo"
	"github.com/tidwall/gjson"
	"html/template"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"os/exec"
	"regexp"
	"strings"
)

var path = "lib/"
var tplPath = "mvn.tpl"
var mvnPath = "mvn.txt"

var client = &http.Client{}

func reqByName(name string) {
	fmt.Println("Hello, world")

	resp, err := http.Get(reqUrl + name)
	if err != nil {
		fmt.Println("http get error", err)
		return
	}
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		fmt.Println("read error", err)
		return
	}
	fmt.Println(string(body))
}
func getCookies() []*http.Cookie {
	var cookies = make([]*http.Cookie, 0, 8)
	var tasks = chromedp.Tasks{
		chromedp.ActionFunc(func(ctx context.Context) error {
			cks, err := network.GetAllCookies().Do(ctx)
			if err != nil {
				return err
			}
			for i, cookie := range cks {
				log.Printf("chrome cookie %d: %+v", i, cookie)
			}
			//network.ClearBrowserCookies()
			return nil
		}),
		chromedp.Navigate("https://mvnrepository.com"),
		chromedp.WaitVisible(`#maincontent`, chromedp.ByID),
		chromedp.ActionFunc(func(ctx context.Context) error {
			cks, err := network.GetAllCookies().Do(ctx)
			for _, ck := range cks {
				cookies = append(cookies, &http.Cookie{
					Name:  ck.Name,
					Value: ck.Value,
				})
			}
			return err
		}),
	}
	err := chromedp.Run(ctx, tasks)
	if err != nil {
		log.Fatal(err)
	}
	return cookies
}

func web(url string, version string) *[]MvnInfo {
	var nodes []*cdp.Node
	var tasks = chromedp.Tasks{
		chromedp.Navigate(url),
		chromedp.WaitVisible(`#maincontent`, chromedp.ByID),
		/*chromedp.ActionFunc(func(ctx context.Context) error {
			cookies, err := network.GetAllCookies().Do(ctx)
			if err != nil {
				return err
			}
			for i, cookie := range cookies {
				log.Printf("chrome cookie %d: %+v", i, cookie)
			}
			return nil
		}),*/
		//xpath语法 a[1]表示第一个
		chromedp.Nodes(`//div[@id='maincontent']//div[@class='im']//p[@class='im-subtitle']/a[2]`, &nodes),
	}
	err := chromedp.Run(ctx, tasks)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("get nodes:", len(nodes))
	// print titles
	var infoList = make([]MvnInfo, 0, 10)
	for _, node := range nodes {
		str := node.Attributes[1]
		groupId := strings.Split(str, "/")[2]
		artifactId := strings.Split(str, "/")[3]
		infoList = append(infoList, MvnInfo{
			GroupId:    groupId,
			ArtifactId: artifactId,
			Version:    version,
		})
	}
	return &infoList
}

type MvnInfo struct {
	GroupId      string
	ArtifactId   string
	Version      string
	Usages       int32
	VersionCount int32
	Jar          string
	Md5          string
	Validate     bool
	Err          string
}

func runChrome(head bool) {
	command := exec.Command("chrome-dev.bat")
	command.Start()
}

func getWs(url string) string {
	req, _ := http.NewRequest("GET", "http://"+url+"/json/list", nil)
	req.Header.Set("User-Agent", userAgent)
	resp, err := client.Do(req)
	if err != nil {
		runChrome(true)
		return getWs(url)
	}
	b, err := ioutil.ReadAll(resp.Body)
	str := string(b)
	str = strings.Replace(str, "\r\n", "", -1)
	str = strings.Replace(str, " ", "", -1)
	var wsUrl = ""
	wsUrl = gjson.Get(str, "..0.0.id").String() //感觉这个是个bug
	var ws = "ws://" + url + "/devtools/page/" + wsUrl
	fmt.Println(ws)
	return ws
}

func cli(name string) *[]MvnInfo {
	var headName = name
	version := ""
	index := findPkgIndex(name)
	if index != -1 { //表示没有找到版本信息
		headName = name[:index]
		version = name[index+1:]
	}
	var fromJar = false
	if version == "" { //从manifast中获取相关版本信息
		version = getVerFromFile(name)
		fromJar = true
	}
	var noVersion = false
	if version == "" {
		noVersion = true
	}
	url := reqUrl + headName
	var infoList = make([]MvnInfo, 0, 10)
	if version == "" {
		return &infoList
	}
	/*resp, err := doReq(url)
	bytes, err := ioutil.ReadAll(resp.Body)
	fmt.Println(string(bytes))
	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		fmt.Println("read error", err)
		return nil
	}

	sel := doc.Find(".im-subtitle")
	if len(sel.Nodes) == 0 {
		panic("cookie过期!")
	}
	sel.Each(func(_ int, s *goquery.Selection) {
		// For each item found, get the band and title
		var info = MvnInfo{}
		s.Find("a").Each(func(i int, s *goquery.Selection) {
			switch i {
			case 0:
				info.GroupId = s.Text()
			case 1:
				info.ArtifactId = s.Text()
			}
		})
		if info.ArtifactId != "" && info.ArtifactId == headName {
			info.Version = version
			info.Jar = name
			infoList = append(infoList, info)
		}
	})
	noneVersionList = append(noneVersionList, MvnInfo{
		Jar: name,
	})*/

	infoList = *web(url, version)
	var idx = -1
	for i, info := range infoList {
		if info.ArtifactId == headName {
			idx = i
			break
		}
	}
	if idx == -1 {
		fmt.Println("noMatch:" + headName)
		infoList[0].Err += ",not_match"
		infoList[0].ArtifactId = headName
		infoList[0].GroupId = ""
	} else if idx > 0 {
		infoList[idx], infoList[0] = infoList[0], infoList[idx]
	}
	if fromJar {
		infoList[0].Err += ",from_jar"
	}
	if noVersion {
		infoList[0].Err += ",no_version"
	}
	return &infoList
}

func getVerFromFile(name string) string {
	manifest, err := jargo.GetManifest(path + name + ".jar")
	if err != nil {
		log.Fatal(err)
		return ""
	}
	version := (*manifest)["Implementation-Version"]
	reg, _ := regexp.Compile(`^\d`)
	indexArr := reg.FindStringIndex(version)
	if len(indexArr) == 0 {
		version = ""
	}
	return version
}

var reqUrl = "https://mvnrepository.com/search?q="
var mvnUrl = "https://repo1.maven.org/maven2/"

var repoUrls = []string{
	"https://repo1.maven.org/maven2/",
	"https://oss.sonatype.org/content/repositories/releases/",
	"https://repo.spring.io/plugins-release/",
	"https://repo.spring.io/libs-milestone/",
	"https://repo.hortonworks.com/content/repositories/releases/",
	"https://repo.jenkins-ci.org/releases/",
}

func getGroupUrl(groupId string) string {
	split := strings.Split(groupId, ".")
	headEl := ""
	for _, it := range split {
		headEl += it + "/"
	}
	return headEl
}
func requestMd5(info *MvnInfo) {
	if info.Version == "" {
		return
	}
	headEl := getGroupUrl(info.GroupId)
	for _, url := range repoUrls {
		if url == mvnUrl {
			md5Uri := url + headEl + info.ArtifactId + "/" + info.Version + "/" +
				info.ArtifactId + "-" + info.Version + ".jar.md5"
			response, _ := http.Get(md5Uri)
			md5, _ := ioutil.ReadAll(response.Body)
			if len(md5) < 50 {
				info.Md5 = string(md5[:32])
				checkMd5(info)
			}
		} else {
			jarUri := url + headEl + info.ArtifactId + "/" + info.Version + "/" +
				info.ArtifactId + "-" + info.Version + ".jar"
			resp, _ := http.Get(jarUri)
			res, _ := GetMd5FromReader(resp.Body)
			if len(res) == 32 {
				info.Md5 = res
				checkMd5(info)

			}
		}
		if info.Validate {
			break
		}
	}

}

func updateState(info *MvnInfo) {
	if info.Version == "" {
		set := findVersion(info)
		for version, _ := range *set {
			info.Version = version
			requestMd5(info)
			if info.Validate {
				break
			}
			info.Md5 = ""
		}
	} else {
		requestMd5(info)
	}
}
func GetMd5FromFile(path string) (string, error) {
	f, err := os.Open(path)
	defer f.Close()
	if err != nil {
		return "", err
	}
	h := md5.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return fmt.Sprintf("%x", h.Sum(nil)), nil
}
func GetMd5FromReader(reader io.Reader) (string, error) {
	h := md5.New()
	if _, err := io.Copy(h, reader); err != nil {
		return "", err
	}
	return fmt.Sprintf("%x", h.Sum(nil)), nil
}

var md5Map = make(map[string]string, 0)

func checkMd5(info *MvnInfo) bool {
	var filePath = path + info.Jar + ".jar"
	md5 := md5Map[filePath]
	if md5 == "" {
		md5, _ = GetMd5FromFile(filePath)
		md5Map[filePath] = md5
	}
	if info.Md5 == md5 {
		info.Validate = true
		return true
	}
	return false
}

func buildMvn() {
	files, err := ioutil.ReadDir(path)
	if err != nil {
		fmt.Println("遍历失败")
	}
	var tplList = make([]MvnInfo, 0, len(files))
	//var badList = make([]MvnInfo,0,len(files))
	//获取模板数据
	tplFile, _ := os.Open(tplPath)
	defer tplFile.Close()
	tpl, _ := ioutil.ReadAll(tplFile)
	template, _ := template.New("mvn").Parse(string(tpl))
	for _, f := range files {
		if f.IsDir() {
			continue
		}
		name := f.Name()

		jarIndex := strings.LastIndex(name, ".jar")
		if jarIndex != -1 {
			name = name[:jarIndex]
			fmt.Println("ready to query: " + name)
			infoList := cli(name)
			if len(*infoList) > 0 {
				//info := getInfo(infoList)
				info := (*infoList)[0]
				versionFormat(&info)
				/*if len(info.Md5) > 0 {
					if !info.Validate {
						info.err = "not_match"
					}
				} else {
					info.err = "not_found"
				}
				if info.Jar == "" {
					info.Jar = name
				}*/
				fmt.Println(info)
				tplList = append(tplList, info)
			}

		}
	}
	//handleNoVersion()
	jarFile, _ := os.OpenFile(mvnPath, os.O_CREATE|os.O_APPEND|os.O_TRUNC, 0666)
	template.Execute(jarFile, tplList)
	jarFile.Close()
}

func findPkgIndex(name string) int {
	reg, _ := regexp.Compile(`-\d`)
	indexArr := reg.FindStringIndex(name)
	index := -1
	if len(indexArr) > 0 {
		index = indexArr[0]
	}
	return index
}

var noneVersionList = make([]MvnInfo, 0, 10)

func getInfo(infoList *[]MvnInfo) *MvnInfo {
	var info = &MvnInfo{}
	for i := 0; i < len(*infoList); i++ {
		mvnInfo := &(*infoList)[i]
		versionFormat(mvnInfo)
		updateState(mvnInfo)
		if mvnInfo.Validate {
			info = mvnInfo
			break
		}
	}
	return info
}
func getInfo2(infoList *[]MvnInfo) *MvnInfo {
	var info = &MvnInfo{}
	for i := 0; i < len(*infoList); i++ {
		mvnInfo := &(*infoList)[i]
		versionFormat(mvnInfo)
		updateState(mvnInfo)
		if mvnInfo.Validate {
			info = mvnInfo
			break
		}
	}
	return info
}
func versionFormat(info *MvnInfo) {
	var replaceMap = make(map[string]string, 0)
	replaceMap["-20140208"] = ""
	replaceMap[".RC1"] = ".RELEASE"
	replaceMap["-r1364789"] = ""

	version := info.Version
	var change = false
	for k, v := range replaceMap {
		if strings.Contains(version, k) {
			change = true
			info.Version = strings.Replace(version, k, v, -1)
			break
		}
	}
	if change {
		info.Err += "ver_old:" + version
	}
}

type VersionInfo struct {
	Repo    string
	Version string
}

func findVersion(info *MvnInfo) *map[string]bool {
	fmt.Println("ready to find version: ", info.Jar)
	var infoSet = make(map[string]bool, 0)
	for _, url := range repoUrls {
		var uri = url + getGroupUrl(info.GroupId) + info.ArtifactId + "/"
		resp, _ := http.Get(uri)
		doc, err := goquery.NewDocumentFromReader(resp.Body)
		if err != nil {
			fmt.Println("read error", err)
		}

		reg, _ := regexp.Compile(`\.\D`)

		doc.Find("a").Each(func(i int, selection *goquery.Selection) {
			version := selection.Text()
			if version != "" {
				indexArr := reg.FindStringIndex(version)
				if len(indexArr) == 0 {
					version = strings.Replace(version, "/", "", -1)
					infoSet[version] = true
				}
			}
		})
	}

	return &infoSet
}
func sortInfo(infoList *[]MvnInfo) {
	for i := 0; i < len(*infoList); i++ {
		for j := i + 1; j < len(*infoList); j++ {
			if (*infoList)[i].VersionCount < (*infoList)[j].VersionCount {
				(*infoList)[i].VersionCount, (*infoList)[j].
					VersionCount = (*infoList)[j].VersionCount, (*infoList)[i].VersionCount
			}
		}
	}
}

var flagDevToolWsUrl = flag.String("devtools-ws-url", "ws://localhost:9222/devtools/page/D026C6B25AA8FC5397E0FC9125BDB894", "DevTools WebSsocket URL")

var userAgent = `Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/84.0.4147.125 Safari/537.36`
var ctx context.Context

func initWeb() {
	opts := append(chromedp.DefaultExecAllocatorOptions[:],
		chromedp.Flag("headless", false), // debug使用
		//chromedp.Flag("blink-settings", "imagesEnabled=false"),
		chromedp.UserAgent(userAgent),
	)

	allocCtx, _ := chromedp.NewExecAllocator(context.Background(), opts...)

	theCtx, _ := chromedp.NewContext(
		allocCtx,
		chromedp.WithLogf(log.Printf),
	)
	ctx = theCtx
}
func initWebRemote() {
	ws := getWs("localhost:9222")
	allocCtx, _ := chromedp.NewRemoteAllocator(context.Background(), ws)
	theCtx, _ := chromedp.NewContext(
		allocCtx,
	)
	ctx = theCtx
}

var cookies = make([]*http.Cookie, 0)

func main() {
	initWebRemote()
	//cookies = getCookies()
	buildMvn()
	//doReq("localhost:9222")
}
func mvnTest() {
	var tplList = make([]MvnInfo, 2)
	tplList = append(tplList, MvnInfo{
		GroupId:    "groupId",
		ArtifactId: "artifactId",
		Version:    "version",
		Jar:        "jar",
		Md5:        "md5",
		Validate:   false,
	})
	tplList = append(tplList, MvnInfo{
		GroupId:    "groupId",
		ArtifactId: "artifactId",
		Version:    "version",
		Jar:        "jar",
		Md5:        "md5",
		Validate:   false,
	})
	tplFile, _ := os.Open(tplPath)
	tpl, _ := ioutil.ReadAll(tplFile)
	template, _ := template.New("mvn").Parse(string(tpl))
	jarFile, _ := os.OpenFile(mvnPath, os.O_CREATE|os.O_APPEND|os.O_TRUNC, 0666)
	template.Execute(jarFile, tplList)
}
