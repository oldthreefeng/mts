package internal

import (
	"context"
	"errors"
	"fmt"
	"io/ioutil"	
	"math/rand"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/axgle/mahonia"
	"github.com/chromedp/cdproto/cdp"
	"github.com/chromedp/cdproto/dom"
	"github.com/chromedp/cdproto/network"
	"github.com/chromedp/cdproto/page"
	"github.com/chromedp/cdproto/target"
	"github.com/chromedp/chromedp"
	"github.com/oldthreefeng/mts/pkg/chrome"
	"github.com/oldthreefeng/mts/pkg/logger"
	"github.com/oldthreefeng/mts/pkg/utils"
	"github.com/tidwall/gjson"
)

// ErrEmptyData is empty date
var ErrEmptyData = errors.New("空数据")

type jdSnap struct {
	ctx         context.Context
	cancel      context.CancelFunc
	bCtx        context.Context
	isLogin     bool
	isClose     bool
	mu          sync.Mutex
	userAgent   string
	UserInfo    gjson.Result
	SkuId       string
	SecKillUrl  string
	SecKillNum  int
	SecKillInfo gjson.Result
	eid         string
	fp          string
	Works       int
	IsOkChan    chan struct{}
	IsOk        bool
	StartTime   time.Time
	DiffTime    int64
	PayPwd string
}

// NewjdSnap is return 
func NewjdSnap(execPath string, skuId string, num, works int) *jdSnap {
	if works < 0 {
		works = 1
	}
	jsk := &jdSnap{
		ctx:        nil,
		isLogin:    false,
		isClose:    false,
		userAgent:  chrome.GetRandUserAgent(),
		SkuId:      skuId,
		SecKillNum: num,
		Works:      works,
		IsOk:       false,
		IsOkChan:   make(chan struct{}, 1),
	}
	jsk.ctx, jsk.cancel = chrome.NewExecCtx(chromedp.ExecPath(execPath), chromedp.UserAgent(jsk.userAgent))
	return jsk
}

func (jsk *jdSnap) SetEid(eid string) {
	jsk.eid = eid
}

func (jsk *jdSnap) SetFp(fp string) {
	jsk.fp = fp
}

func (jsk *jdSnap) Stop() {
	jsk.mu.Lock()
	defer jsk.mu.Unlock()
	if jsk.isClose {
		return
	}
	jsk.isClose = true
	c := jsk.cancel
	c()
	return
}

func (jsk *jdSnap) GetReq(reqUrl string, params map[string]string, referer string, ctx context.Context, isDisableRedirects bool) (gjson.Result, error) {
	if referer == "" {
		referer = "https://www.jd.com"
	}
	if ctx == nil {
		ctx = jsk.bCtx
	}
	req, _ := http.NewRequest("GET", reqUrl, nil)
	req.Header.Add("User-Agent", jsk.userAgent)
	req.Header.Add("Referer", referer)
	req.Header.Add("Host", req.URL.Host)
	if len(params) > 0 {
		q := req.URL.Query()
		for k, v := range params {
			q.Add(k, v)
		}
		req.URL.RawQuery = q.Encode()
	}
	resp, err := chrome.RequestByCookie(ctx, req, isDisableRedirects)
	if err != nil {
		return gjson.Result{}, err
	}
	if resp.StatusCode != 200 {
		logger.Info("httpCode: ", resp.StatusCode, "reqUrl: ", resp.Request.URL)
	}
	//设置cookie到浏览器
	for _, respCookie := range resp.Cookies() {
		ok, err := network.SetCookie(respCookie.Name, respCookie.Value).WithURL(resp.Request.URL.String()).Do(ctx)
		if !ok {
			logger.Error(respCookie.Name, respCookie.Value, " cookie设置失败", err)
		}
	}
	defer resp.Body.Close()
	b, _ := ioutil.ReadAll(resp.Body)
	logger.Info("Get请求接口:", req.URL)
	//	logger.Debug(string(b))
	logger.Info("=======================")
	r := utils.FormatJsonpResponse(b, req.URL.String(), false)
	if r.Raw == "null" || r.Raw == "" {
		return gjson.Result{}, ErrEmptyData
	}
	return r, nil
}

func (jsk *jdSnap) SyncJdTime() {
	resp, err := http.Get("https://a.jd.com//ajax/queryServerData.html")
	if err != nil {
		logger.Error(err)
		os.Exit(0)
		return
	}
	defer resp.Body.Close()
	b, _ := ioutil.ReadAll(resp.Body)
	r := gjson.ParseBytes(b)
	jdTimeUnix := r.Get("serverTime").Int()
	jsk.DiffTime = utils.UnixMilli() - jdTimeUnix
	logger.Info("服务器与本地时间差为: ", jsk.DiffTime, "ms")
}

func (jsk *jdSnap) PostReq(reqUrl string, params url.Values, referer string, ctx context.Context, isDisableRedirects bool) (gjson.Result, error) {
	if ctx == nil {
		ctx = jsk.bCtx
	}
	req, _ := http.NewRequest("POST", reqUrl, strings.NewReader(params.Encode()))
	req.Header.Add("User-Agent", jsk.userAgent)
	if referer != "" {
		req.Header.Add("Referer", referer)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Add("Host", req.URL.Host)
	resp, err := chrome.RequestByCookie(ctx, req, isDisableRedirects)
	if err != nil {
		return gjson.Result{}, err
	}

	if resp.StatusCode != 200 {
		logger.Warn("httpCode: ", resp.StatusCode, "reqUrl: ", resp.Request.URL)
	}
	//设置cookie到浏览器
	for _, respCookie := range resp.Cookies() {
		_, _ = network.SetCookie(respCookie.Name, respCookie.Value).WithURL(resp.Request.URL.String()).Do(ctx)
	}
	defer resp.Body.Close()
	b, _ := ioutil.ReadAll(resp.Body)

	logger.Info("Post请求连接", req.URL)
	logger.Info("=======================")
	r := utils.FormatJsonpResponse(b, req.URL.String(), false)
	if r.Raw == "null" || r.Raw == "" {
		return gjson.Result{}, ErrEmptyData
	}
	return r, nil
}

func FormatJdResponse(b []byte, prefix string, isConvertStr bool) gjson.Result {
	r := string(b)
	if isConvertStr {
		r = mahonia.NewDecoder("gbk").ConvertString(r)
	}
	r = strings.TrimSpace(r)
	if prefix != "" {
		//这里针对http连接 自动提取jsonp的callback
		if strings.HasPrefix(prefix, "http") {
			pUrl, err := url.Parse(prefix)
			if err == nil {
				prefix = pUrl.Query().Get("callback")
			}
		}
		r = strings.TrimPrefix(r, prefix)
	}
	if strings.HasSuffix(r, ")") {
		r = strings.TrimLeft(r, `(`)
		r = strings.TrimRight(r, ")")
	}
	return gjson.Parse(r)
}

//初始化监听请求数据
func (jsk *jdSnap) InitActionFunc() chromedp.ActionFunc {
	return func(ctx context.Context) error {
		jsk.bCtx = ctx
		_ = network.Enable().Do(ctx)
		chromedp.ListenTarget(ctx, func(ev interface{}) {
			switch e := ev.(type) {
			case *network.EventResponseReceived:
				go func() {
					if strings.Contains(e.Response.URL, "passport.jd.com/user/petName/getUserInfoForMiniJd.action") {
						b, err := network.GetResponseBody(e.RequestID).Do(ctx)
						if err == nil {
							jsk.UserInfo = FormatJdResponse(b, e.Response.URL, false)
						}
						jsk.isLogin = true
					}
				}()

			}
		})
		return nil
	}
}

func (jsk *jdSnap) Run() error {
	return chromedp.Run(jsk.ctx, chromedp.Tasks{
		jsk.InitActionFunc(),
		chromedp.Navigate("https://passport.jd.com/uc/login"),
		chromedp.ActionFunc(func(ctx context.Context) error {
			logger.Info("等待登陆......")
			for {
				select {
				case <-jsk.ctx.Done():
					logger.Error("浏览器被关闭，退出进程")
					return nil
				case <-jsk.bCtx.Done():
					logger.Error("浏览器被关闭，退出进程")
					return nil
				default:
				}
				if jsk.isLogin {
					logger.Debug(jsk.UserInfo.Get("realName").String() + ", 登陆成功........")
					break
				}
			}
			return nil
		}),
		jsk.GetEidAndFp(),
		chromedp.ActionFunc(func(ctx context.Context) error {
			u := "https://item.jd.com/" + jsk.SkuId + ".html"
			rand.Seed(time.Now().UnixNano())
			_ = chromedp.Navigate(u).Do(ctx)
			for i := 0; i < jsk.Works; i++ {
				go func() {
					jsk.WaitStart()
					for {
						jsk.FetchSecKillUrl()
						logger.Info("正在访问抢购连接......")
						_, err := jsk.GetReq(jsk.SecKillUrl, nil, "https://item.jd.com/"+jsk.SkuId+".html", jsk.bCtx, true)
						//这里访问会响应302 禁止重定向后就会是空数据 所以这里空数据是正常的
						if err == nil || err.Error() == ErrEmptyData.Error() {
							break
						}
					}
					SecKillRE:
					//请求抢购连接，提交订单
					err := jsk.ReqSubmitSecKillOrder(jsk.bCtx)
					if err != nil {
						logger.Info(err, "等待重试")
						i := rand.Intn(200)
						time.Sleep(time.Duration(i) * time.Millisecond)
						goto SecKillRE
					}
					_ = chromedp.Navigate("https://order.jd.com/center/list.action").Do(jsk.bCtx)
				}()
			}
			select {
			case <-jsk.IsOkChan:
				logger.Info("抢购成功。。。10s后关闭进程...")
				_ = chromedp.Sleep(10 * time.Second).Do(ctx)
			case <-jsk.ctx.Done():
			case <-jsk.bCtx.Done():
			}
			return nil
		}),
	})
}

func (jsk *jdSnap) WaitStart() {
	st := jsk.StartTime.UnixNano() / 1e6
	logger.Info("等待时间到达" + jsk.StartTime.Format(utils.DateTimeFormatStr) + "...... 请勿关闭浏览器")
	for {
		select {
		case <-jsk.ctx.Done():
			logger.Error("浏览器被关闭，退出进程")
			return
		case <-jsk.bCtx.Done():
			logger.Error("浏览器被关闭，退出进程")
			return
		default:
		}
		d := utils.UnixMilli()-jsk.DiffTime
		if d >= st {
			logger.Info("时间到达。。。。开始执行", time.Now().Format(utils.DateTimeFormatStr))
			break
		}
		if st - d - 4 > 0 {
			time.Sleep(time.Duration(st - d - 4) * time.Millisecond)
		}
	}
}

func (jsk *jdSnap) GetEidAndFp() chromedp.ActionFunc {
	return func(ctx context.Context) error {
		logger.Info(jsk.fp, jsk.eid)
		if jsk.eid != "" && jsk.fp != "" {
			logger.Info("已传入eid与fp，程序将不再自动获取 ")
			logger.Info("eid : ", jsk.eid, "fp : ", jsk.fp)
			return nil
		}
	RE:
		logger.Info("正在获取eid和fp参数....")
		_ = chromedp.Navigate("https://search.jd.com/Search?keyword=衣服").Do(ctx)
		logger.Info("等待页面更新完成....")
		_ = chromedp.WaitVisible(".gl-item").Do(ctx)
		var itemNodes []*cdp.Node
		err := chromedp.Nodes(".gl-item", &itemNodes, chromedp.ByQueryAll).Do(ctx)
		if err != nil {
			return err
		}
		n := itemNodes[rand.Intn(len(itemNodes))]
		_ = dom.ScrollIntoViewIfNeeded().WithNodeID(n.NodeID).Do(ctx)
		_, _, _, _ = page.Navigate("https://item.jd.com/" + n.AttributeValue("data-sku") + ".html").Do(ctx)

		logger.Info("等待商品详情页更新完成....")
		_ = chromedp.WaitVisible("#InitCartUrl").Do(ctx)
		_ = chromedp.Sleep(1 * time.Second).Do(ctx)
		_ = chromedp.Click("#InitCartUrl").Do(ctx)
		_ = chromedp.WaitVisible("#GotoShoppingCart").Do(ctx)
		_ = chromedp.Sleep(1 * time.Second).Do(ctx)
		_ = chromedp.Click("#GotoShoppingCart").Do(ctx)
		//_ = chromedp.Navigate("https://cart.jd.com/cart_index/").Do(ctx)
		ch, cc := chrome.WaitDocumentUpdated(ctx)
		logger.Info("等待购物车页面.....")
		<-ch
		cc()
		info, _ := target.GetTargetInfo().Do(ctx)
		if strings.Contains(info.URL, "cart.jd.com/cart_index") {
			logger.Info("Click, common-submit-btn")
			_ = chromedp.Sleep(1 * time.Second).Do(ctx);
			_ = chromedp.Click(".common-submit-btn").Do(ctx)
		} else {
			logger.Info("Click, submit-btn")
			_ = chromedp.WaitVisible("container", chromedp.ByID).Do(ctx)
			_ = chromedp.ScrollIntoView(".submit-btn").Do(ctx);
			_ = chromedp.Sleep(1 * time.Second).Do(ctx);
			_ = chromedp.Click(".submit-btn").Do(ctx)
		}

		//_ = chromedp.WaitVisible("#mainframe").Do(ctx)
		ch, cc = chrome.WaitDocumentUpdated(ctx)
		logger.Info("等待结算页加载完成..... 如遇到未选中商品错误，可手动选中后点击结算")
		<-ch
		cc()
		//执行js参数 将eid和fp显示到对应元素上
		_ = chromedp.Sleep(3 * time.Second).Do(ctx)
		res := make(map[string]interface{})
		err = chromedp.Evaluate("_JdTdudfp", &res).Do(ctx)
		logger.Error(err)
		logger.Info("_JdTdudfp: ", res)
		eid, ok := res["eid"]
		if !ok {
			logger.Info("获取eid失败,正在重试")
			goto RE
		}
		jsk.eid = eid.(string)
		jsk.fp = res["fp"].(string)

		if jsk.fp == "" || jsk.eid == "" || jsk.fp == "undefined" || jsk.eid == "undefined" {
			logger.Warn("获取参数失败，等待重试。。。 重试过程过久可手动刷新浏览器")
			goto RE
		}
		logger.Info("参数获取成功：eid【" + jsk.eid + "】, fp【" + jsk.fp + "】")

		return nil
	}
}

func (jsk *jdSnap) FetchSecKillUrl() {
	/*jsk.SecKillUrl = "https://marathon.jd.com/captcha.html?skuId="+jsk.SkuId+"&sn=c3f4ececd8461f0e4d7267e96a91e0e0&from=pc"
	return*/
	logger.Info("开始获取抢购连接.....")
	for {
		if jsk.SecKillUrl != "" {
			break
		}
		jsk.SecKillUrl = jsk.GetSecKillUrl()
		logger.Warn("抢购链接获取失败.....正在重试")
	}
	jsk.SecKillUrl = "https:" + strings.TrimPrefix(jsk.SecKillUrl, "https:")
	jsk.SecKillUrl = strings.ReplaceAll(jsk.SecKillUrl, "divide", "marathon")
	jsk.SecKillUrl = strings.ReplaceAll(jsk.SecKillUrl, "user_routing", "captcha.html")
	logger.Debug("抢购连接获取成功....", jsk.SecKillUrl)
	return
}

func (jsk *jdSnap) ReqSubmitSecKillOrder(ctx context.Context) error {
	if ctx == nil {
		ctx = jsk.bCtx
	}

	defer func() {
		if r := recover(); r != nil {
			logger.Error(r)
		}
	}()
	//这里修改为直接使用http请求访问抢购结算页面 提高速度
	skUrl := fmt.Sprintf("https://marathon.jd.com/seckill/seckill.action?skuId=%s&num=%d&rid=%d", jsk.SkuId, jsk.SecKillNum, time.Now().Unix())
	logger.Info("访问抢购订单结算页面......", skUrl)
	_, _ = jsk.GetReq(skUrl, nil, "https://item.jd.com/"+jsk.SkuId+".html", ctx, true)

	//这里直接使用浏览器跳转 主要目的是获取cookie
	/*jsk.GetReq(skUrl, nil, "https://item.jd.com/"+jsk.SkuId+".html", ctx)
	_, _, _, _ = page.Navigate(skUrl).WithReferrer("https://item.jd.com/"+jsk.SkuId+".html").Do(ctx)*/

	logger.Info("获取抢购信息...............")
	err := jsk.GetSecKillInitInfo(ctx)
	if err != nil {
		logger.Error("抢购失败：", err, "正在重试.......")
		return err
	}

	orderData := jsk.GetOrderReqData()

	if len(orderData) == 0 {
		return errors.New("订单参数生成失败")
	}
	logger.Info("订单参数：", orderData.Encode())
	logger.Info("提交抢购订单.............")

	r, err := jsk.PostReq("https://marathon.jd.com/seckillnew/orderService/pc/submitOrder.action?skuId="+jsk.SkuId+"", orderData, skUrl, ctx, false)
	if err != nil {
		logger.Error("订单提交失败，正在重新提交.....", " errMsg => ", err, " raw => ", r.Raw)
		return err
	}
	orderId := r.Get("orderId").String()
	if orderId != "" && orderId != "0" {
		jsk.IsOk = true
		jsk.IsOkChan <- struct{}{}
		logger.Info("抢购成功，订单编号:", r.Get("orderId").String())
	} else {
		if r.IsObject() || r.IsArray() {
			return errors.New("抢购失败：" + r.Raw)
		}
		return errors.New("抢购失败,再接再厉")
	}
	return nil
}

func (jsk *jdSnap) GetOrderReqData() url.Values {
	logger.Info("生成订单所需参数...")
	defer func() {
		if f := recover(); f != nil {
			logger.Error("订单参数错误：", f)
		}
	}()

	addressList := jsk.SecKillInfo.Get("addressList").Array()
	var defaultAddress gjson.Result
	for _, dAddress := range addressList {
		if dAddress.Get("defaultAddress").Bool() {
			logger.Info("获取到默认收货地址")
			defaultAddress = dAddress
		}
	}
	if defaultAddress.Raw == "" {
		logger.Info("没有获取到默认收货地址， 自动选择一个地址")
		defaultAddress = addressList[0]
	}
	invoiceInfo := jsk.SecKillInfo.Get("invoiceInfo")
	r := url.Values{
		"skuId":              []string{jsk.SkuId},
		"num":                []string{strconv.Itoa(jsk.SecKillNum)},
		"addressId":          []string{defaultAddress.Get("id").String()},
		"yuShou":             []string{"true"},
		"isModifyAddress":    []string{"false"},
		"name":               []string{defaultAddress.Get("name").String()},
		"provinceId":         []string{defaultAddress.Get("provinceId").String()},
		"cityId":             []string{defaultAddress.Get("cityId").String()},
		"countyId":           []string{defaultAddress.Get("countyId").String()},
		"townId":             []string{defaultAddress.Get("townId").String()},
		"addressDetail":      []string{defaultAddress.Get("addressDetail").String()},
		"mobile":             []string{defaultAddress.Get("mobile").String()},
		"mobileKey":          []string{defaultAddress.Get("mobileKey").String()},
		"email":              []string{defaultAddress.Get("email").String()},
		"postCode":           []string{""},
		"invoiceTitle":       []string{""},
		"invoiceCompanyName": []string{""},
		"invoiceContent":     []string{},
		"invoiceTaxpayerNO":  []string{""},
		"invoiceEmail":       []string{""},
		"invoicePhone":       []string{invoiceInfo.Get("invoicePhone").String()},
		"invoicePhoneKey":    []string{invoiceInfo.Get("invoicePhoneKey").String()},
		"invoice":            []string{"true"},
		"password":           []string{jsk.PayPwd},
		"codTimeType":        []string{"3"},
		"paymentType":        []string{"4"},
		"areaCode":           []string{""},
		"overseas":           []string{"0"},
		"phone":              []string{""},
		"eid":                []string{jsk.eid},
		"fp":                 []string{jsk.fp},
		"token":              []string{jsk.SecKillInfo.Get("token").String()},
		"pru":                []string{""},
	}

	if invoiceInfo.Raw == "" {
		r["invoice"] = []string{"false"}
	} else {
		r["invoice"] = []string{"true"}
	}
	t := invoiceInfo.Get("invoiceTitle").String()
	if t != "" {
		r["invoiceTitle"] = []string{t}
	} else {
		r["invoiceTitle"] = []string{"-1"}
	}

	t = invoiceInfo.Get("invoiceContentType").String()
	if t != "" {
		r["invoiceContent"] = []string{t}
	} else {
		r["invoiceContent"] = []string{"1"}
	}

	return r
}

func (jsk *jdSnap) GetSecKillInitInfo(ctx context.Context) error {
	r, err := jsk.PostReq("https://marathon.jd.com/seckillnew/orderService/pc/init.action", url.Values{
		"sku":             []string{jsk.SkuId},
		"num":             []string{strconv.Itoa(jsk.SecKillNum)},
		"isModifyAddress": []string{"false"},
	}, fmt.Sprintf("https://marathon.jd.com/seckill/seckill.action?skuId=%s&num=%d&rid=%d", jsk.SkuId, jsk.SecKillNum, time.Now().Unix()), ctx, false)
	if err != nil {
		return err
	}
	jsk.SecKillInfo = r
	logger.Info("秒杀信息获取成功：", jsk.SecKillInfo.Raw)
	return nil
}

func (jsk *jdSnap) GetSecKillUrl() string {
	r, _ := jsk.GetReq("https://itemko.jd.com/itemShowBtn", map[string]string{
		"callback": "jQuery" + strconv.FormatInt(utils.GenerateRangeNum(1000000, 9999999), 10),
		"skuId":    jsk.SkuId,
		"from":     "pc",
		"_":        strconv.FormatInt(time.Now().Unix()*1000, 10),
	}, "https://item.jd.com/"+jsk.SkuId+".html", nil, false)
	return r.Get("url").String()
}
