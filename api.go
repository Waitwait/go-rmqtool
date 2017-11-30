package rmqtool

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"strings"
	"time"
)

var (
	DefaultClientTimeout               time.Duration = 3 * time.Second //请求超时
	DefaultTransportInsecureSkipVerify bool          = false           //是否忽略ssl验证
	DefaultTransportDisableCompression bool          = true            //是否禁止压缩
)

type APIClient struct {
	api    string
	user   string
	passwd string
}

func NewAPIClient(api, user, passwd string) *APIClient {
	return &APIClient{
		api:    api,
		user:   user,
		passwd: passwd,
	}
}

func (c *APIClient) SetApi(api string) *APIClient {
	c.api = api
	return c
}

func (c *APIClient) SetAuth(user, passwd string) *APIClient {
	c.user = user
	c.passwd = passwd
	return c
}

func (c *APIClient) GET(paths interface{}) (*http.Response, error) {
	return c.do("GET", paths, nil)
}

func (c *APIClient) PUT(paths interface{}, data interface{}) (*http.Response, error) {
	return c.do("PUT", paths, data)
}

func (c *APIClient) POST(paths interface{}, data interface{}) (*http.Response, error) {
	return c.do("POST", paths, data)
}

func (c *APIClient) DELETE(paths interface{}) (*http.Response, error) {
	return c.do("DELETE", paths, nil)
}

func (c *APIClient) NewRequest(method string, paths interface{}, data interface{}) (*http.Request, error) {
	api, err := c.url(paths)
	if err != nil {
		return nil, err
	}
	buffer, err := c.buffer(data)
	if err != nil {
		return nil, err
	}
	request, err := http.NewRequest(strings.ToUpper(method), api, buffer)
	if err != nil {
		return nil, err
	}
	request.Header.Add("Content-Type", "application/json")
	request.Header.Add("X-Tool", "GoRMQTool")
	return request, nil
}

func (c *APIClient) Do(req *http.Request) (*http.Response, error) {
	req.SetBasicAuth(c.user, c.passwd)
	return c.client().Do(req)
}

func (c *APIClient) do(method string, paths interface{}, data interface{}) (*http.Response, error) {
	request, err := c.NewRequest(method, paths, data)
	if err != nil {
		return nil, err
	}
	return c.Do(request)
}

func (c *APIClient) buffer(data interface{}) (*bytes.Buffer, error) {
	var buffer *bytes.Buffer
	switch data.(type) {
	case nil:
		buffer = bytes.NewBufferString("")
	case string:
		buffer = bytes.NewBufferString(data.(string))
	case []byte:
		buffer = bytes.NewBuffer(data.([]byte))
	default:
		b, err := json.Marshal(data)
		if err != nil {
			return nil, err
		}
		buffer = bytes.NewBuffer(b)
	}
	return buffer, nil
}

func (c *APIClient) url(paths interface{}) (string, error) {
	var realPath string
	switch paths.(type) {
	case []string:
		tmp := make([]string, len(paths.([]string)))
		for i, v := range paths.([]string) {
			tmp[i] = url.PathEscape(v)
		}
		realPath = strings.Join(tmp, "/")
	case string:
		realPath = paths.(string)
	case []byte:
		realPath = string(paths.([]byte))
	default:
		return "", fmt.Errorf("path error: %v", paths)
	}
	return strings.TrimRight(c.api, "/") + "/" + strings.TrimLeft(realPath, "/"), nil
}

func (c *APIClient) client() *http.Client {
	client := &http.Client{
		Timeout: DefaultClientTimeout,
	}
	URL, err := url.Parse(c.api)
	if err != nil && strings.ToLower(URL.Scheme) == "https" {
		client.Transport = &http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: DefaultTransportInsecureSkipVerify,
			},
			DisableCompression: DefaultTransportDisableCompression,
		}
	}
	return &http.Client{}
}

func (c *APIClient) ScanByte(resp *http.Response, err error) ([]byte, error) {
	if err != nil {
		return nil, err
	} else if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API Response Status Error: %d, %v", resp.StatusCode, resp)
	} else {
		defer resp.Body.Close()
		return ioutil.ReadAll(resp.Body)
	}
}

func (c *APIClient) ScanMap(resp *http.Response, err error) (map[string]interface{}, error) {
	b, err := c.ScanByte(resp, err)
	if err != nil {
		return nil, err
	}
	var ret map[string]interface{}
	err = json.Unmarshal(b, &ret)
	return ret, err
}

func (c *APIClient) ScanSlice(resp *http.Response, err error) ([]map[string]interface{}, error) {
	b, err := c.ScanByte(resp, err)
	if err != nil {
		return nil, err
	}
	var ret []map[string]interface{}
	err = json.Unmarshal(b, &ret)
	return ret, err

}

func (c *APIClient) read(paths interface{}) ([]byte, error) {
	resp, err := c.GET(paths)
	return c.ScanByte(resp, err)
}

func (c *APIClient) readMap(paths interface{}) (map[string]interface{}, error) {
	resp, err := c.GET(paths)
	return c.ScanMap(resp, err)
}

func (c *APIClient) readSlice(paths interface{}) ([]map[string]interface{}, error) {
	resp, err := c.GET(paths)
	return c.ScanSlice(resp, err)
}

func (c *APIClient) create(paths interface{}, data interface{}) error {
	resp, err := c.PUT(paths, data)
	if err != nil {
		return err
	} else if (resp.StatusCode == http.StatusNoContent) || (resp.StatusCode == http.StatusCreated) {
		// enusre response code
		// created success OR already create
		return nil
	} else {
		return fmt.Errorf("API Response Status Error: %d, %v", resp.StatusCode, resp)
	}
}

func (c *APIClient) update(paths interface{}, data interface{}) error {
	// ensure binding
	resp, err := c.POST(paths, data)
	if err != nil {
		return err
	}
	if (resp.StatusCode == http.StatusNoContent) || (resp.StatusCode == http.StatusCreated) {
		// enusre response code
		return nil
	} else {
		return fmt.Errorf("API Response Status Error: %d, %v", resp.StatusCode, resp)
	}
}

func (c *APIClient) ScanDelete(resp *http.Response, err error) error {
	if err != nil {
		return err
	} else if resp.StatusCode == http.StatusNoContent {
		// enusre response code
		return nil
	} else {
		return fmt.Errorf("API Response Status Error: %d, %v", resp.StatusCode, resp)
	}
}

func (c *APIClient) delete(paths interface{}) error {
	resp, err := c.DELETE(paths)
	return c.ScanDelete(resp, err)
}

func (c *APIClient) Check() error {
	_, err := c.GET("")
	return err
}

func (c *APIClient) ListQueues(vhost string) ([]map[string]interface{}, error) {
	if vhost == "" {
		return c.readSlice("/queues")
	} else {
		return c.readSlice([]string{"queues", vhost})
	}
}

func (c *APIClient) CreateQueue(vhost, name string, params map[string]interface{}) error {
	// `{"auto_delete":false, "durable":true, "arguments":[]}`
	return c.create([]string{"queues", vhost, name}, params)
}

func (c *APIClient) DeleteQueue(vhost, name string) error {
	return c.delete([]string{"queues", vhost, name})
}

func (c *APIClient) PurgeQueue(vhost, name string) error {
	return c.delete([]string{"queues", vhost, name, "contents"})
}

func (c *APIClient) BindingsRoutingKey(vhost, name, exchange, key string, args map[string]interface{}) error {
	params := map[string]interface{}{
		"routing_key": key,
		"arguments":   args,
	}
	return c.update([]string{"bindings", vhost, "e", exchange, "q", name}, params)
}

func APIListQueues(api, user, passwd, vhost string) ([]map[string]interface{}, error) {
	return NewAPIClient(api, user, passwd).ListQueues(vhost)
}

func APICreateQueue(api, user, passwd, vhost, name string, params map[string]interface{}) error {
	return NewAPIClient(api, user, passwd).CreateQueue(vhost, name, params)
}

func APIDeleteQueue(api, user, passwd, vhost, name string) error {
	return NewAPIClient(api, user, passwd).DeleteQueue(vhost, name)
}

func APIPurgeQueue(api, user, passwd, vhost, name string) error {
	return NewAPIClient(api, user, passwd).PurgeQueue(vhost, name)
}

func APIBindingsRoutingKey(api, user, passwd, vhost, name, exchange, key string, args map[string]interface{}) error {
	return NewAPIClient(api, user, passwd).BindingsRoutingKey(vhost, name, exchange, key, args)
}

func RegisterQueue(api, user, passwd, vhost, name, exchange string, keys []string) error {
	err := APICreateQueue(api, user, passwd, vhost, name, nil)
	if err != nil {
		return err
	}
	for _, key := range keys {
		if exchange != "" && key != "" {
			err := APIBindingsRoutingKey(api, user, passwd, vhost, name, exchange, key, nil)
			if err != nil {
				return err
			}
		}
	}
	return nil
}
