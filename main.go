package main

import (
	"context"
	"crypto/md5"
	"fmt"
	"github.com/PuerkitoBio/goquery"
	"github.com/chromedp/cdproto/cdp"
	"github.com/chromedp/cdproto/network"
	"github.com/gnewton/jargo"
	"html/template"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"regexp"
	"strings"
)

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

type MvnInfo struct {
	GroupId      string
	ArtifactId   string
	Version      string
	Usages       int32
	VersionCount int32
	Jar          string
	Md5          string
	Validate     bool
	err          string
}

func cli(name string) *[]MvnInfo {
	var headName = name
	version := ""
	index := findPkgIndex(name)
	if index != -1 { //表示没有找到版本信息
		headName = name[:index]
		version = name[index+1:]
	}
	if version == "" { //从manifast中获取相关版本信息
		version = getVerFromFile(name)
	}
	var infoList = make([]MvnInfo, 0, 10)
	if version == "" {
		return &infoList
	}
	client := &http.Client{}
	req, _ := http.NewRequest("GET", reqUrl+headName, nil)
	req.Header.Set("User-Agent",
		"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/84.0.4147.105 Safari/537.36")
	req.Header.Set("cookie",
		"__cfduid=d824af0f6d448feddf63e3820f38c86d41596259564; _ga=GA1.2.714444043.1596259579; _gid=GA1.2.1104784891.1596259579; cf_clearance=769d6c5e799616c9d3f6d8fa322e3f329ba8745c-1596379785-0-1zef0c033azb041d86bzce278e85-150; MVN_SESSION=eyJhbGciOiJIUzI1NiJ9.eyJkYXRhIjp7InVpZCI6IjdmNjY3YjQxLWQzYjctMTFlYS05MTFmLTgxMzA0MjZkMGU4MCJ9LCJleHAiOjE2Mjc5MTY3NjMsIm5iZiI6MTU5NjM4MDc2MywiaWF0IjoxNTk2MzgwNzYzfQ.IPa-nxy9vxGS5DCrQ1NK8omwauaSTU3JTU5P4CET4xA")
	resp, err := client.Do(req)
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
	})
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

/*var repoUrls =map[string]string{
	"Central":"https://repo1.maven.org/maven2/",
	"Sonatype":"https://oss.sonatype.org/content/repositories/releases/",
	"Spring Plugins":"https://repo.spring.io/plugins-release/",
	"Spring Lib M":"https://repo.spring.io/libs-milestone/",
	"Hortonworks": "https://repo.hortonworks.com/content/repositories/releases/",
	"Jenkins":"https://repo.jenkins-ci.org/releases/",
}*/

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

var path = "lib/"
var tplPath = "mvn.tpl"
var mvnPath = "mvn.txt"

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
			println("ready to query: " + name)
			infoList := cli(name)
			if len(*infoList) > 0 {
				info := getInfo(infoList)
				if len(info.Md5) > 0 {
					if !info.Validate {
						info.err = "not_match"
					}
				} else {
					info.err = "not_found"
				}
				if info.Jar == "" {
					info.Jar = name
				}
				fmt.Println(*info)
				tplList = append(tplList, *info)
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

func versionFormat(info *MvnInfo) {
	var replaceMap = make(map[string]string, 0)
	replaceMap["-20140208"] = ""
	version := info.Version
	for k, v := range replaceMap {
		version = strings.Replace(version, k, v, -1)
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
func main() {
	buildMvn()
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
