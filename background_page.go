package playwright

import (
	"encoding/base64"
	"fmt"
	"io/ioutil"
	"reflect"
)

type backgroundPageImpl struct {
	channelOwner
	isClosed        bool
	video           *videoImpl
	mouse           *mouseImpl
	keyboard        *keyboardImpl
	touchscreen     *touchscreenImpl
	timeoutSettings *timeoutSettings
	browserContext  *browserContextImpl
	frames          []Frame
	workers         []Worker
	mainFrame       Frame
	routes          []*routeHandlerEntry
	viewportSize    ViewportSize
	ownedContext    BrowserContext
	bindings        map[string]BindingCallFunction
}

func (p *backgroundPageImpl) Context() BrowserContext {
	return p.browserContext
}

func (p *backgroundPageImpl) Close(options ...PageCloseOptions) error {
	_, err := p.channel.Send("close", options)
	if err != nil {
		return err
	}
	if p.ownedContext != nil {
		return p.ownedContext.Close()
	}
	return nil
}

func (p *backgroundPageImpl) InnerText(selector string, options ...PageInnerTextOptions) (string, error) {
	return p.mainFrame.InnerText(selector, options...)
}

func (p *backgroundPageImpl) InnerHTML(selector string, options ...PageInnerHTMLOptions) (string, error) {
	return p.mainFrame.InnerHTML(selector, options...)
}

func (p *backgroundPageImpl) Opener() (Page, error) {
	channel := p.initializer["opener"]
	channelOwner := fromNullableChannel(channel)
	if channelOwner == nil {
		return nil, nil
	}
	return channelOwner.(*backgroundPageImpl), nil
}

func (p *backgroundPageImpl) MainFrame() Frame {
	return p.mainFrame
}

// PageFrameOptions is the option struct for Page.Frame()
type PageFrameOptions struct {
	Name *string
	URL  interface{}
}

func (p *backgroundPageImpl) Frame(options PageFrameOptions) Frame {
	var matcher *urlMatcher
	if options.URL != nil {
		matcher = newURLMatcher(options.URL)
	}

	for _, f := range p.frames {
		if options.Name != nil && f.Name() == *options.Name {
			return f
		}

		if options.URL != nil && matcher != nil && matcher.Matches(f.URL()) {
			return f
		}
	}

	return nil
}

func (p *backgroundPageImpl) Frames() []Frame {
	return p.frames
}

func (p *backgroundPageImpl) SetDefaultNavigationTimeout(timeout float64) {
	p.timeoutSettings.SetNavigationTimeout(timeout)
	p.channel.SendNoReply("setDefaultNavigationTimeoutNoReply", map[string]interface{}{
		"timeout": timeout,
	})
}

func (p *backgroundPageImpl) SetDefaultTimeout(timeout float64) {
	p.timeoutSettings.SetTimeout(timeout)
	p.channel.SendNoReply("setDefaultTimeoutNoReply", map[string]interface{}{
		"timeout": timeout,
	})
}

func (p *backgroundPageImpl) QuerySelector(selector string) (ElementHandle, error) {
	return p.mainFrame.QuerySelector(selector)
}

func (p *backgroundPageImpl) QuerySelectorAll(selector string) ([]ElementHandle, error) {
	return p.mainFrame.QuerySelectorAll(selector)
}

func (p *backgroundPageImpl) WaitForSelector(selector string, options ...PageWaitForSelectorOptions) (ElementHandle, error) {
	return p.mainFrame.WaitForSelector(selector, options...)
}

func (p *backgroundPageImpl) DispatchEvent(selector string, typ string, options ...PageDispatchEventOptions) error {
	return p.mainFrame.DispatchEvent(selector, typ, nil, options...)
}

func (p *backgroundPageImpl) Evaluate(expression string, options ...interface{}) (interface{}, error) {
	return p.mainFrame.Evaluate(expression, options...)
}

func (p *backgroundPageImpl) EvaluateHandle(expression string, options ...interface{}) (JSHandle, error) {
	return p.mainFrame.EvaluateHandle(expression, options...)
}

func (p *backgroundPageImpl) EvalOnSelector(selector string, expression string, options ...interface{}) (interface{}, error) {
	return p.mainFrame.EvalOnSelector(selector, expression, options...)
}

func (p *backgroundPageImpl) EvalOnSelectorAll(selector string, expression string, options ...interface{}) (interface{}, error) {
	return p.mainFrame.EvalOnSelectorAll(selector, expression, options...)
}

func (p *backgroundPageImpl) AddScriptTag(options PageAddScriptTagOptions) (ElementHandle, error) {
	return p.mainFrame.AddScriptTag(options)
}

func (p *backgroundPageImpl) AddStyleTag(options PageAddStyleTagOptions) (ElementHandle, error) {
	return p.mainFrame.AddStyleTag(options)
}

func (p *backgroundPageImpl) SetExtraHTTPHeaders(headers map[string]string) error {
	_, err := p.channel.Send("setExtraHTTPHeaders", map[string]interface{}{
		"headers": serializeMapToNameAndValue(headers),
	})
	return err
}

func (p *backgroundPageImpl) URL() string {
	return p.mainFrame.URL()
}

func (p *backgroundPageImpl) Unroute(url interface{}, handlers ...routeHandler) error {
	p.Lock()
	defer p.Unlock()

	routes, err := unroute(p.channel, p.routes, url, handlers...)
	if err != nil {
		return err
	}
	p.routes = routes

	return nil
}

func (p *backgroundPageImpl) Content() (string, error) {
	return p.mainFrame.Content()
}

func (p *backgroundPageImpl) SetContent(content string, options ...PageSetContentOptions) error {
	return p.mainFrame.SetContent(content, options...)
}

func (p *backgroundPageImpl) Goto(url string, options ...PageGotoOptions) (Response, error) {
	return p.mainFrame.Goto(url, options...)
}

func (p *backgroundPageImpl) Reload(options ...PageReloadOptions) (Response, error) {
	response, err := p.channel.Send("reload", options)
	if err != nil {
		return nil, err
	}
	return fromChannel(response).(*responseImpl), err
}

func (p *backgroundPageImpl) WaitForLoadState(state ...string) {
	p.mainFrame.WaitForLoadState(state...)
}

func (p *backgroundPageImpl) GoBack(options ...PageGoBackOptions) (Response, error) {
	channel, err := p.channel.Send("goBack", options)
	if err != nil {
		return nil, err
	}
	channelOwner := fromNullableChannel(channel)
	if channelOwner == nil {
		return nil, nil
	}
	return channelOwner.(*responseImpl), nil
}

func (p *backgroundPageImpl) GoForward(options ...PageGoForwardOptions) (Response, error) {
	resp, err := p.channel.Send("goForward", options)
	if err != nil {
		return nil, err
	}
	obj := fromNullableChannel(resp)
	if obj == nil {
		return nil, nil
	}
	return obj.(*responseImpl), nil
}

func (p *backgroundPageImpl) EmulateMedia(options ...PageEmulateMediaOptions) error {
	_, err := p.channel.Send("emulateMedia", options)
	if err != nil {
		return err
	}
	return err
}

// ViewportSize represents the viewport size
type ViewportSize struct {
	Width  int `json:"width"`
	Height int `json:"height"`
}

func (p *backgroundPageImpl) SetViewportSize(width, height int) error {
	_, err := p.channel.Send("setViewportSize", map[string]interface{}{
		"viewportSize": map[string]interface{}{
			"width":  width,
			"height": height,
		},
	})
	if err != nil {
		return err
	}
	p.viewportSize.Width = width
	p.viewportSize.Height = height
	return nil
}

func (p *backgroundPageImpl) ViewportSize() ViewportSize {
	return p.viewportSize
}

func (p *backgroundPageImpl) BringToFront() error {
	_, err := p.channel.Send("bringToFront")
	return err
}

func (p *backgroundPageImpl) Type(selector, text string, options ...PageTypeOptions) error {
	return p.mainFrame.Type(selector, text, options...)
}

func (p *backgroundPageImpl) Fill(selector, text string, options ...FrameFillOptions) error {
	return p.mainFrame.Fill(selector, text, options...)
}

func (p *backgroundPageImpl) Press(selector, key string, options ...PagePressOptions) error {
	return p.mainFrame.Press(selector, key, options...)
}

func (p *backgroundPageImpl) Title() (string, error) {
	return p.mainFrame.Title()
}

func (p *backgroundPageImpl) Workers() []Worker {
	return p.workers
}

func (p *backgroundPageImpl) Screenshot(options ...PageScreenshotOptions) ([]byte, error) {
	var path *string
	if len(options) > 0 {
		path = options[0].Path
	}
	data, err := p.channel.Send("screenshot", options)
	if err != nil {
		return nil, fmt.Errorf("could not send message :%w", err)
	}
	image, err := base64.StdEncoding.DecodeString(data.(string))
	if err != nil {
		return nil, fmt.Errorf("could not decode base64 :%w", err)
	}
	if path != nil {
		if err := ioutil.WriteFile(*path, image, 0644); err != nil {
			return nil, err
		}
	}
	return image, nil
}

func (p *backgroundPageImpl) PDF(options ...PagePdfOptions) ([]byte, error) {
	var path *string
	if len(options) > 0 {
		path = options[0].Path
	}
	data, err := p.channel.Send("pdf", options)
	if err != nil {
		return nil, fmt.Errorf("could not send message :%w", err)
	}
	pdf, err := base64.StdEncoding.DecodeString(data.(string))
	if err != nil {
		return nil, fmt.Errorf("could not decode base64 :%w", err)
	}
	if path != nil {
		if err := ioutil.WriteFile(*path, pdf, 0644); err != nil {
			return nil, err
		}
	}
	return pdf, nil
}

func (p *backgroundPageImpl) Click(selector string, options ...PageClickOptions) error {
	return p.mainFrame.Click(selector, options...)
}

func (p *backgroundPageImpl) WaitForEvent(event string, predicate ...interface{}) interface{} {
	return <-waitForEvent(p, event, predicate...)
}

func (p *backgroundPageImpl) WaitForNavigation(options ...PageWaitForNavigationOptions) (Response, error) {
	return p.mainFrame.WaitForNavigation(options...)
}

func (p *backgroundPageImpl) WaitForRequest(url interface{}, options ...interface{}) Request {
	var matcher *urlMatcher
	if url != nil {
		matcher = newURLMatcher(url)
	}
	predicate := func(req *requestImpl) bool {
		if matcher != nil {
			return matcher.Matches(req.URL())
		}
		if len(options) == 1 {
			return reflect.ValueOf(options[0]).Call([]reflect.Value{reflect.ValueOf(req)})[0].Bool()
		}
		return true
	}
	return p.WaitForEvent("request", predicate).(*requestImpl)
}

func (p *backgroundPageImpl) WaitForResponse(url interface{}, options ...interface{}) Response {
	var matcher *urlMatcher
	if url != nil {
		matcher = newURLMatcher(url)
	}
	predicate := func(req *responseImpl) bool {
		if matcher != nil {
			return matcher.Matches(req.URL())
		}
		if len(options) == 1 {
			return reflect.ValueOf(options[0]).Call([]reflect.Value{reflect.ValueOf(req)})[0].Bool()
		}
		return true
	}
	return p.WaitForEvent("response", predicate).(*responseImpl)
}

func (p *backgroundPageImpl) ExpectEvent(event string, cb func() error, predicates ...interface{}) (interface{}, error) {
	args := []interface{}{event}
	if len(predicates) == 1 {
		args = append(args, predicates[0])
	}
	return newExpectWrapper(p.WaitForEvent, args, cb)
}

func (p *backgroundPageImpl) ExpectNavigation(cb func() error, options ...PageWaitForNavigationOptions) (Response, error) {
	navigationOptions := make([]interface{}, 0)
	for _, option := range options {
		navigationOptions = append(navigationOptions, option)
	}
	response, err := newExpectWrapper(p.WaitForNavigation, navigationOptions, cb)
	if response == nil {
		return nil, err
	}
	return response.(*responseImpl), err
}

func (p *backgroundPageImpl) ExpectConsoleMessage(cb func() error) (ConsoleMessage, error) {
	consoleMessage, err := newExpectWrapper(p.WaitForEvent, []interface{}{"console"}, cb)
	return consoleMessage.(*consoleMessageImpl), err
}

func (p *backgroundPageImpl) ExpectedDialog(cb func() error) (Dialog, error) {
	dialog, err := newExpectWrapper(p.WaitForEvent, []interface{}{"dialog"}, cb)
	return dialog.(*dialogImpl), err
}

func (p *backgroundPageImpl) ExpectDownload(cb func() error) (Download, error) {
	download, err := newExpectWrapper(p.WaitForEvent, []interface{}{"download"}, cb)
	return download.(*downloadImpl), err
}

func (p *backgroundPageImpl) ExpectFileChooser(cb func() error) (FileChooser, error) {
	response, err := newExpectWrapper(p.WaitForEvent, []interface{}{"filechooser"}, cb)
	return response.(*fileChooserImpl), err
}

func (p *backgroundPageImpl) ExpectLoadState(state string, cb func() error) error {
	_, err := newExpectWrapper(p.mainFrame.WaitForLoadState, []interface{}{state}, cb)
	return err
}

func (p *backgroundPageImpl) ExpectPopup(cb func() error) (Page, error) {
	popup, err := newExpectWrapper(p.WaitForEvent, []interface{}{"popup"}, cb)
	return popup.(*backgroundPageImpl), err
}

func (p *backgroundPageImpl) ExpectResponse(url interface{}, cb func() error, options ...interface{}) (Response, error) {
	response, err := newExpectWrapper(p.WaitForResponse, append([]interface{}{url}, options...), cb)
	if err != nil {
		return nil, err
	}
	return response.(*responseImpl), err
}

func (p *backgroundPageImpl) ExpectRequest(url interface{}, cb func() error, options ...interface{}) (Request, error) {
	popup, err := newExpectWrapper(p.WaitForRequest, append([]interface{}{url}, options...), cb)
	if err != nil {
		return nil, err
	}
	return popup.(*requestImpl), err
}

func (p *backgroundPageImpl) ExpectWorker(cb func() error) (Worker, error) {
	response, err := newExpectWrapper(p.WaitForEvent, []interface{}{"worker"}, cb)
	return response.(*workerImpl), err
}

func (p *backgroundPageImpl) Route(url interface{}, handler routeHandler) error {
	p.Lock()
	defer p.Unlock()
	p.routes = append(p.routes, newRouteHandlerEntry(newURLMatcher(url), handler))
	if len(p.routes) == 1 {
		_, err := p.channel.Send("setNetworkInterceptionEnabled", map[string]interface{}{
			"enabled": true,
		})
		if err != nil {
			return err
		}
	}
	return nil
}

func (p *backgroundPageImpl) GetAttribute(selector string, name string, options ...PageGetAttributeOptions) (string, error) {
	return p.mainFrame.GetAttribute(selector, name, options...)
}

func (p *backgroundPageImpl) Hover(selector string, options ...PageHoverOptions) error {
	return p.mainFrame.Hover(selector, options...)
}

func (p *backgroundPageImpl) IsClosed() bool {
	return p.isClosed
}

func (p *backgroundPageImpl) AddInitScript(options PageAddInitScriptOptions) error {
	var source string
	if options.Script != nil {
		source = *options.Script
	}
	if options.Path != nil {
		content, err := ioutil.ReadFile(*options.Path)
		if err != nil {
			return err
		}
		source = string(content)
	}
	_, err := p.channel.Send("addInitScript", map[string]interface{}{
		"source": source,
	})
	return err
}

func (p *backgroundPageImpl) Keyboard() Keyboard {
	return p.keyboard
}
func (p *backgroundPageImpl) Mouse() Mouse {
	return p.mouse
}

func (p *backgroundPageImpl) Touchscreen() Touchscreen {
	return p.touchscreen
}

func (p *backgroundPageImpl) setBrowserContext(browserContext *browserContextImpl) {
	p.browserContext = browserContext
	p.timeoutSettings = newTimeoutSettings(browserContext.timeoutSettings)
}

func newPage(parent *channelOwner, objectType string, guid string, initializer map[string]interface{}) *backgroundPageImpl {
	bt := &backgroundPageImpl{
		mainFrame: fromChannel(initializer["mainFrame"]).(*frameImpl),
		workers:   make([]Worker, 0),
		routes:    make([]*routeHandlerEntry, 0),
		bindings:  make(map[string]BindingCallFunction),
		viewportSize: ViewportSize{
			Height: int(initializer["viewportSize"].(map[string]interface{})["height"].(float64)),
			Width:  int(initializer["viewportSize"].(map[string]interface{})["width"].(float64)),
		},
		timeoutSettings: newTimeoutSettings(nil),
	}
	bt.frames = []Frame{bt.mainFrame}
	bt.mainFrame.(*frameImpl).page = bt
	bt.createChannelOwner(bt, parent, objectType, guid, initializer)
	bt.mouse = newMouse(bt.channel)
	bt.keyboard = newKeyboard(bt.channel)
	bt.touchscreen = newTouchscreen(bt.channel)
	bt.channel.On("bindingCall", func(params map[string]interface{}) {
		bt.onBinding(fromChannel(params["binding"]).(*bindingCallImpl))
	})
	bt.channel.On("close", bt.onClose)
	bt.channel.On("console", func(ev map[string]interface{}) {
		bt.Emit("console", fromChannel(ev["message"]))
	})
	bt.channel.On("crash", func() {
		bt.Emit("crash")
	})
	bt.channel.On("dialog", func(ev map[string]interface{}) {
		go func() {
			bt.Emit("dialog", fromChannel(ev["dialog"]))
		}()
	})
	bt.channel.On("domcontentloaded", func() {
		bt.Emit("domcontentloaded")
	})
	bt.channel.On("fileChooser", func(ev map[string]interface{}) {
		bt.Emit("filechooser", newFileChooser(bt, fromChannel(ev["element"]).(*elementHandleImpl), ev["isMultiple"].(bool)))
	})
	bt.channel.On("frameAttached", func(ev map[string]interface{}) {
		bt.onFrameAttached(fromChannel(ev["frame"]).(*frameImpl))
	})
	bt.channel.On("frameDetached", func(ev map[string]interface{}) {
		bt.onFrameDetached(fromChannel(ev["frame"]).(*frameImpl))
	})
	bt.channel.On(
		"load",
		func(params map[string]interface{}) {
			bt.Emit("load")
		},
	)
	bt.channel.On(
		"pageError",
		func(params map[string]interface{}) {
			err := errorPayload{}
			remapMapToStruct(params["error"].(map[string]interface{})["error"], &err)
			bt.Emit("pageerror", parseError(err))
		},
	)
	bt.channel.On("popup", func(ev map[string]interface{}) {
		bt.Emit("popup", fromChannel(ev["page"]))
	})
	bt.channel.On("route", func(ev map[string]interface{}) {
		bt.onRoute(fromChannel(ev["route"]).(*routeImpl), fromChannel(ev["request"]).(*requestImpl))
	})
	bt.channel.On("download", func(ev map[string]interface{}) {
		url := ev["url"].(string)
		suggestedFilename := ev["suggestedFilename"].(string)
		artifact := fromChannel(ev["artifact"]).(*artifactImpl)
		bt.Emit("download", newDownload(bt, url, suggestedFilename, artifact))
	})
	bt.channel.On("video", func(params map[string]interface{}) {
		bt.Video().(*videoImpl).setArtifact(fromChannel(params["artifact"]).(*artifactImpl))
	})
	bt.channel.On("webSocket", func(ev map[string]interface{}) {
		bt.Emit("websocket", fromChannel(ev["webSocket"]).(*webSocketImpl))
	})

	bt.channel.On("worker", func(ev map[string]interface{}) {
		bt.onWorker(fromChannel(ev["worker"]).(*workerImpl))
	})
	bt.addEventHandler(func(name string, handler interface{}) {
		if name == "filechooser" && bt.ListenerCount(name) == 0 {
			bt.channel.SendNoReply("setFileChooserInterceptedNoReply", map[string]interface{}{
				"intercepted": true,
			})
		}
	})
	bt.removeEventHandler(func(name string, handler interface{}) {
		if name == "filechooser" && bt.ListenerCount(name) == 0 {
			bt.channel.SendNoReply("setFileChooserInterceptedNoReply", map[string]interface{}{
				"intercepted": false,
			})
		}
	})

	return bt
}

func (p *backgroundPageImpl) onBinding(binding *bindingCallImpl) {
	function := p.bindings[binding.initializer["name"].(string)]
	if function == nil {
		return
	}
	go binding.Call(function)
}

func (p *backgroundPageImpl) onFrameAttached(frame *frameImpl) {
	frame.page = p
	p.frames = append(p.frames, frame)
	p.Emit("frameattached", frame)
}

func (p *backgroundPageImpl) onFrameDetached(frame *frameImpl) {
	frame.detached = true
	frames := make([]Frame, 0)
	for i := 0; i < len(p.frames); i++ {
		if p.frames[i] != frame {
			frames = append(frames, frame)
		}
	}
	if len(frames) != len(p.frames) {
		p.frames = frames
	}
	p.Emit("framedetached", frame)
}

func (p *backgroundPageImpl) onRoute(route *routeImpl, request *requestImpl) {
	go func() {
		for _, handlerEntry := range p.routes {
			if handlerEntry.matcher.Matches(request.URL()) {
				handlerEntry.handler(route, request)
				return
			}
		}
		p.browserContext.onRoute(route, request)
	}()
}

func (p *backgroundPageImpl) onWorker(worker *workerImpl) {
	p.workers = append(p.workers, worker)
	worker.page = p
	p.Emit("worker", worker)
}

func (p *backgroundPageImpl) onClose() {
	p.isClosed = true
	newPages := []Page{}
	p.browserContext.Lock()
	for _, page := range p.browserContext.pages {
		if page != p {
			newPages = append(newPages, page)
		}
	}
	p.browserContext.pages = newPages
	p.browserContext.Unlock()
	p.Emit("close")
}

func (p *backgroundPageImpl) SetInputFiles(selector string, files []InputFile, options ...FrameSetInputFilesOptions) error {
	return p.mainFrame.SetInputFiles(selector, files, options...)
}

func (p *backgroundPageImpl) Check(selector string, options ...FrameCheckOptions) error {
	return p.mainFrame.Check(selector, options...)
}

func (p *backgroundPageImpl) Uncheck(selector string, options ...FrameUncheckOptions) error {
	return p.mainFrame.Uncheck(selector, options...)
}

func (p *backgroundPageImpl) WaitForTimeout(timeout float64) {
	p.mainFrame.WaitForTimeout(timeout)
}

func (p *backgroundPageImpl) WaitForFunction(expression string, arg interface{}, options ...FrameWaitForFunctionOptions) (JSHandle, error) {
	return p.mainFrame.WaitForFunction(expression, arg, options...)
}

func (p *backgroundPageImpl) Dblclick(expression string, options ...FrameDblclickOptions) error {
	return p.mainFrame.Dblclick(expression, options...)
}

func (p *backgroundPageImpl) Focus(expression string, options ...FrameFocusOptions) error {
	return p.mainFrame.Focus(expression, options...)
}

func (p *backgroundPageImpl) TextContent(selector string, options ...FrameTextContentOptions) (string, error) {
	return p.mainFrame.TextContent(selector, options...)
}

func (p *backgroundPageImpl) Video() Video {
	if p.video == nil {
		p.video = newVideo(p)
	}
	return p.video
}

func (p *backgroundPageImpl) Tap(selector string, options ...FrameTapOptions) error {
	return p.mainFrame.Tap(selector, options...)
}

func (p *backgroundPageImpl) ExposeFunction(name string, binding ExposedFunction) error {
	return p.ExposeBinding(name, func(source *BindingSource, args ...interface{}) interface{} {
		return binding(args...)
	})
}
func (p *backgroundPageImpl) ExposeBinding(name string, binding BindingCallFunction, handle ...bool) error {
	needsHandle := false
	if len(handle) == 1 {
		needsHandle = handle[0]
	}
	if _, ok := p.bindings[name]; ok {
		return fmt.Errorf("Function '%s' has been already registered", name)
	}
	if _, ok := p.browserContext.bindings[name]; ok {
		return fmt.Errorf("Function '%s' has been already registered in the browser context", name)
	}
	p.bindings[name] = binding
	_, err := p.channel.Send("exposeBinding", map[string]interface{}{
		"name":        name,
		"needsHandle": needsHandle,
	})
	return err
}

func (p *backgroundPageImpl) SelectOption(selector string, values SelectOptionValues, options ...FrameSelectOptionOptions) ([]string, error) {
	return p.mainFrame.SelectOption(selector, values, options...)
}

func (p *backgroundPageImpl) IsChecked(selector string, options ...FrameIsCheckedOptions) (bool, error) {
	return p.mainFrame.IsChecked(selector, options...)
}

func (p *backgroundPageImpl) IsDisabled(selector string, options ...FrameIsDisabledOptions) (bool, error) {
	return p.mainFrame.IsDisabled(selector, options...)
}

func (p *backgroundPageImpl) IsEditable(selector string, options ...FrameIsEditableOptions) (bool, error) {
	return p.mainFrame.IsEditable(selector, options...)
}

func (p *backgroundPageImpl) IsEnabled(selector string, options ...FrameIsEnabledOptions) (bool, error) {
	return p.mainFrame.IsEnabled(selector, options...)
}

func (p *backgroundPageImpl) IsHidden(selector string, options ...FrameIsHiddenOptions) (bool, error) {
	return p.mainFrame.IsHidden(selector, options...)
}

func (p *backgroundPageImpl) IsVisible(selector string, options ...FrameIsVisibleOptions) (bool, error) {
	return p.mainFrame.IsVisible(selector, options...)
}

func (p *backgroundPageImpl) DragAndDrop(source, target string, options ...FrameDragAndDropOptions) error {
	return p.mainFrame.DragAndDrop(source, target, options...)
}

func (p *backgroundPageImpl) Pause() error {
	return p.browserContext.Pause()
}

func (p *backgroundPageImpl) InputValue(selector string, options ...FrameInputValueOptions) (string, error) {
	return p.mainFrame.InputValue(selector, options...)
}

func (p *backgroundPageImpl) WaitForURL(url string, options ...FrameWaitForURLOptions) error {
	return p.mainFrame.WaitForURL(url, options...)
}
