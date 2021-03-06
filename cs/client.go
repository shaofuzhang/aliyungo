package cs

import (
	"bytes"
	"crypto/md5"
	"encoding/base64"
	"encoding/json"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"time"

	"github.com/51idc/aliyungo/common"
	"github.com/51idc/aliyungo/util"
)

const (
	// CRMDefaultEndpoint is the default API endpoint of CRM services
	CSDefaultEndpoint = "https://cs.aliyuncs.com"
	CSAPIVersion      = "2015-12-15"
)

// The Client type encapsulates operations with an OSS region.
type Client struct {
	AccessKeyId     string
	AccessKeySecret string
	endpoint        string
	Version         string
	debug           bool
	httpClient      *http.Client
}

type Response struct {
	RequestId string `json:"request_id"`
}

// NewClient creates a new instance of CRM client
func NewClient(accessKeyId, accessKeySecret string) *Client {
	return &Client{
		AccessKeyId:     accessKeyId,
		AccessKeySecret: accessKeySecret,
		endpoint:        CSDefaultEndpoint,
		Version:         CSAPIVersion,
		httpClient:      &http.Client{},
	}
}

// SetDebug sets debug mode to log the request/response message
func (client *Client) SetDebug(debug bool) {
	client.debug = debug
}

type Request struct {
	Method          string
	URL             string
	Version         string
	Region          common.Region
	Signature       string
	SignatureMethod string
	SignatureNonce  string
	Timestamp       util.ISO6801Time
	Body            []byte
}

// Invoke sends the raw HTTP request for ECS services
func (client *Client) Invoke(region common.Region, method string, path string, query url.Values, args interface{}, response interface{}) error {

	var reqBody []byte
	var err error
	var contentType string
	var contentMD5 string

	if args != nil {
		reqBody, err = json.Marshal(args)
		if err != nil {
			return err
		}
		contentType = "application/json"
		hasher := md5.New()
		hasher.Write(reqBody)
		contentMD5 = base64.StdEncoding.EncodeToString(hasher.Sum(nil))
	}

	requestURL := client.endpoint + path
	if query != nil && len(query) > 0 {
		requestURL = requestURL + "?" + util.Encode(query)
	}
	var bodyReader io.Reader
	if reqBody != nil {
		bodyReader = bytes.NewReader(reqBody)
	}
	httpReq, err := http.NewRequest(method, requestURL, bodyReader)
	if err != nil {
		return common.GetClientError(err)
	}

	if region != "" {
		httpReq.Header.Set("x-acs-region-id", string(region))
	}

	if contentType != "" {
		httpReq.Header.Set("Content-Type", contentType)
	}
	if contentMD5 != "" {
		httpReq.Header.Set("Content-MD5", contentMD5)
	}
	// TODO move to util and add build val flag
	httpReq.Header.Set("Date", util.GetGMTime())
	httpReq.Header.Set("Accept", "application/json")
	//httpReq.Header.Set("x-acs-version", client.Version)
	httpReq.Header.Set("x-acs-signature-version", "1.0")
	httpReq.Header.Set("x-acs-signature-nonce", util.CreateRandomString())
	httpReq.Header.Set("x-acs-signature-method", "HMAC-SHA1")

	client.signRequest(httpReq)

	t0 := time.Now()
	httpResp, err := client.httpClient.Do(httpReq)
	t1 := time.Now()
	if err != nil {
		return common.GetClientError(err)
	}
	statusCode := httpResp.StatusCode

	if client.debug {
		log.Printf("Invoke %s %s %d (%v)", method, requestURL, statusCode, t1.Sub(t0))
	}

	defer httpResp.Body.Close()
	body, err := ioutil.ReadAll(httpResp.Body)

	if err != nil {
		return common.GetClientError(err)
	}

	if client.debug {
		var prettyJSON bytes.Buffer
		err = json.Indent(&prettyJSON, body, "", "    ")
		log.Println(string(prettyJSON.Bytes()))
	}

	if statusCode >= 400 && statusCode <= 599 {
		errorResponse := common.ErrorResponse{}
		err = json.Unmarshal(body, &errorResponse)
		ecsError := &common.Error{
			ErrorResponse: errorResponse,
			StatusCode:    statusCode,
		}
		return ecsError
	}

	if response != nil {
		err = json.Unmarshal(body, response)
		//log.Printf("%++v", response)
		if err != nil {
			return common.GetClientError(err)
		}
	}

	return nil
}
