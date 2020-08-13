package moni

import (
	"context"
	"fmt"
	"github.com/chromedp/cdproto/cdp"
	"github.com/chromedp/cdproto/network"
	"github.com/chromedp/chromedp"
	"time"
)

// 任务 主要用来设置cookie ，获取登录账号后的页面
func VisitWeb(url string, cookies ...string) chromedp.Tasks {
	//创建一个chrome任务
	return chromedp.Tasks{
		//ActionFunc是一个适配器，允许使用普通函数作为操作。
		chromedp.ActionFunc(func(ctx context.Context) error {
			// 设置Cookie存活时间
			expr := cdp.TimeSinceEpoch(time.Now().Add(180 * 24 * time.Hour))
			// 添加Cookie到chrome
			for i := 0; i < len(cookies); i += 2 {
				//SetCookie使用给定的cookie数据设置一个cookie； 如果存在，可能会覆盖等效的cookie。
				success, err := network.SetCookie(cookies[i], cookies[i+1]).
					// 设置cookie到期时间
					WithExpires(&expr).
					// 设置cookie作用的站点
					WithDomain("dl.xzg01.com:83"). //访问网站主体
					// 设置httponly,防止XSS攻击
					WithHTTPOnly(true).
					//Do根据提供的上下文执行Network.setCookie。
					Do(ctx)
				if err != nil {
					return err
				}
				if !success {
					return fmt.Errorf("could not set cookie %q to %q", cookies[i], cookies[i+1])
				}
			}
			return nil
		}),
		// 跳转指定的url地址
		chromedp.Navigate(url),
	}
}

func DoCrawler(res *string) chromedp.Tasks {
	return chromedp.Tasks{
		//下面注释掉的 Navigate 不要随便添加，如果添加上每次执行都相当于刷新，这样就永远翻不了页
		//chromedp.Navigate("http://dl.xzg01.com:83/OpRoot/MemberScoreList.aspx?uid=0&op=0&uname=003008"),
		chromedp.Sleep(1000),                             // 等待
		chromedp.WaitVisible(`#form1`, chromedp.ByQuery), //等待id=from1页面可见  ByQuery是使用DOM选择器查找
		chromedp.Sleep(2 * time.Second),
		// Click 是元素查询操作，它将鼠标单击事件发送到与选择器匹配的第一个元素节点。
		chromedp.Click(`.pagination li:nth-last-child(4) a`, chromedp.ByQuery), //点击翻页
		chromedp.OuterHTML(`tbody`, res, chromedp.ByQuery),                     //获取tbody标签的html
	}
}
