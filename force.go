package simpleforce

import (
	"encoding/xml"
	"fmt"
	"github.com/pkg/errors"
	"io/ioutil"
	"log"
	"net/http"
	"strings"
)

const (
	DefaultAPIVersion = "43.0"
	DefaultClientID   = "simpleforce"
	DefaultURL        = "https://login.salesforce.com"

	logPrefix = "[simpleforce]"
)

var (
	// ErrFailure is a generic error if none of the other errors are appropriate.
	ErrFailure = errors.New("general failure")
)

// Client is the main instance to access salesforce.
type Client struct {
	sessionID string
	user      struct {
		id       string
		name     string
		fullName string
		email    string
	}
	clientID   string
	apiVersion string
	baseURL    string
}

// NewClient creates a new instance of the client.
func NewClient(url, clientID, apiVersion string) *Client {
	client := &Client{
		apiVersion: apiVersion,
		baseURL:    url,
		clientID:   clientID,
	}
	return client
}

// LoginPassword signs into salesforce using password.
// Ref: https://developer.salesforce.com/docs/atlas.en-us.214.0.api_rest.meta/api_rest/intro_understanding_username_password_oauth_flow.htm
// Ref: https://developer.salesforce.com/docs/atlas.en-us.214.0.api.meta/api/sforce_api_calls_login.htm
func (client *Client) LoginPassword(username, password, token string) error {
    // Use the SOAP interface to acquire session ID with username, password, and token.
    // Do not use REST interface here as REST interface seems to have strong checking against client_id, while the SOAP
    // interface allows a non-exist placeholder client_id to be used.
	soapBody := `<?xml version="1.0" encoding="utf-8" ?>
        <env:Envelope
                xmlns:xsd="http://www.w3.org/2001/XMLSchema"
                xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance"
                xmlns:env="http://schemas.xmlsoap.org/soap/envelope/"
                xmlns:urn="urn:partner.soap.sforce.com">
            <env:Header>
                <urn:CallOptions>
                    <urn:client>%s</urn:client>
                    <urn:defaultNamespace>sf</urn:defaultNamespace>
                </urn:CallOptions>
            </env:Header>
            <env:Body>
                <n1:login xmlns:n1="urn:partner.soap.sforce.com">
                    <n1:username>%s</n1:username>
                    <n1:password>%s%s</n1:password>
                </n1:login>
            </env:Body>
        </env:Envelope>`
	soapBody = fmt.Sprintf(soapBody, client.clientID, username, password, token)

	url := fmt.Sprintf("%s/services/Soap/u/%s", client.baseURL, client.apiVersion)
	req, err := http.NewRequest(http.MethodPost, url, strings.NewReader(soapBody))
	if err != nil {
		log.Println(logPrefix, "error occurred creating request,", err)
		return err
	}
	req.Header.Add("Content-Type", "text/xml")
	req.Header.Add("charset", "UTF-8")
	req.Header.Add("SOAPAction", "login")

	httpClient := &http.Client{}
	resp, err := httpClient.Do(req)
	if err != nil {
		log.Println(logPrefix, "error occurred submitting request,", err)
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		log.Println(logPrefix, "request failed,", resp.StatusCode)
		return ErrFailure
	}

	respData, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Println(logPrefix, "error occurred reading response data,", err)
	}

	var loginResponse struct {
		XMLName      xml.Name `xml:"Envelope"`
		SessionID    string   `xml:"Body>loginResponse>result>sessionId"`
		UserID       string   `xml:"Body>loginResponse>result>userId"`
		UserEmail    string   `xml:"Body>loginResponse>result>userInfo>userEmail"`
		UserFullName string   `xml:"Body>loginResponse>result>userInfo>userFullName"`
		UserName     string   `xml:"Body>loginResponse>result>userInfo>userName"`
	}

	err = xml.Unmarshal(respData, &loginResponse)
	if err != nil {
		log.Println(logPrefix, "error occurred parsing login response,", err)
		return err
	}

	// Now we should all be good and the sessionID can be used to talk to salesforce further.
	client.sessionID = loginResponse.SessionID
	client.user.id = loginResponse.UserID
	client.user.name = loginResponse.UserName
	client.user.email = loginResponse.UserEmail
	client.user.fullName = loginResponse.UserFullName

	log.Println(logPrefix, "user", client.user.name, "logged in.")
	return nil
}
