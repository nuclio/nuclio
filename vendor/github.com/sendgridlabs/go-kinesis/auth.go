package kinesis

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"strings"
	"time"
)

const (
	AccessEnvKey        = "AWS_ACCESS_KEY"
	AccessEnvKeyId      = "AWS_ACCESS_KEY_ID"
	SecretEnvKey        = "AWS_SECRET_KEY"
	SecretEnvAccessKey  = "AWS_SECRET_ACCESS_KEY"
	SecurityTokenEnvKey = "AWS_SECURITY_TOKEN"

	AWSMetadataServer = "169.254.169.254"
	AWSIAMCredsPath   = "/latest/meta-data/iam/security-credentials"
	AWSIAMCredsURL    = "http://" + AWSMetadataServer + "/" + AWSIAMCredsPath
)

// Auth interface for authentication credentials and information
type Auth interface {
	GetToken() string
	GetExpiration() time.Time
	GetSecretKey() string
	GetAccessKey() string
	HasExpiration() bool
	Renew() error
	Sign(*Service, time.Time) []byte
}

// AuthCredentials holds the AWS credentials and metadata
type AuthCredentials struct {
	// accessKey, secretKey are the standard AWS auth credentials
	accessKey, secretKey, token string

	// expiry indicates the time at which these credentials expire. If this is set
	// to anything other than the zero value, indicates that the credentials are
	// temporary (and probably fetched from an IAM role from the metadata server)
	expiry time.Time
}

// NewAuth creates a *AuthCredentials struct that adheres to the Auth interface to
// dynamically retrieve AWS credentials
func NewAuth(accessKey, secretKey, token string) *AuthCredentials {
	return &AuthCredentials{
		accessKey: accessKey,
		secretKey: secretKey,
		token:     token,
	}
}

// NewAuthFromEnv retrieves auth credentials from environment vars
func NewAuthFromEnv() (*AuthCredentials, error) {
	accessKey := os.Getenv(AccessEnvKey)
	if accessKey == "" {
		accessKey = os.Getenv(AccessEnvKeyId)
	}

	secretKey := os.Getenv(SecretEnvKey)
	if secretKey == "" {
		secretKey = os.Getenv(SecretEnvAccessKey)
	}

	token := os.Getenv(SecurityTokenEnvKey)

	if accessKey == "" && secretKey == "" && token == "" {
		return nil, fmt.Errorf("No access key (%s or %s), secret key (%s or %s), or security token (%s) env variables were set", AccessEnvKey, AccessEnvKeyId, SecretEnvKey, SecretEnvAccessKey, SecurityTokenEnvKey)
	}
	if accessKey == "" {
		return nil, fmt.Errorf("Unable to retrieve access key from %s or %s env variables", AccessEnvKey, AccessEnvKeyId)
	}
	if secretKey == "" {
		return nil, fmt.Errorf("Unable to retrieve secret key from %s or %s env variables", SecretEnvKey, SecretEnvAccessKey)
	}

	return NewAuth(accessKey, secretKey, token), nil
}

// NewAuthFromMetadata retrieves auth credentials from the metadata
// server. If an IAM role is associated with the instance we are running on, the
// metadata server will expose credentials for that role under a known endpoint.
//
// TODO: specify custom network (connect, read) timeouts, else this will block
// for the default timeout durations.
func NewAuthFromMetadata() (*AuthCredentials, error) {
	auth := &AuthCredentials{}
	if err := auth.Renew(); err != nil {
		return nil, err
	}
	return auth, nil
}

// HasExpiration returns true if the expiration time is non-zero and false otherwise
func (a *AuthCredentials) HasExpiration() bool {
	return !a.expiry.IsZero()
}

// GetExpiration retrieves the current expiration time
func (a *AuthCredentials) GetExpiration() time.Time {
	return a.expiry
}

// GetToken returns the token
func (a *AuthCredentials) GetToken() string {
	return a.token
}

// GetSecretKey returns the secret key
func (a *AuthCredentials) GetSecretKey() string {
	return a.secretKey
}

// GetAccessKey returns the access key
func (a *AuthCredentials) GetAccessKey() string {
	return a.accessKey
}

// Renew retrieves a new token and mutates it on an instance of the Auth struct
func (a *AuthCredentials) Renew() error {
	role, err := retrieveIAMRole()
	if err != nil {
		return err
	}

	data, err := retrieveAWSCredentials(role)
	if err != nil {
		return err
	}

	// Ignore the error, it just means we won't be able to refresh the
	// credentials when they expire.
	expiry, _ := time.Parse(time.RFC3339, data["Expiration"])

	a.expiry = expiry
	a.accessKey = data["AccessKeyId"]
	a.secretKey = data["SecretAccessKey"]
	a.token = data["Token"]
	return nil
}

// Sign API request by
// http://docs.amazonwebservices.com/general/latest/gr/signature-version-4.html

func (a *AuthCredentials) Sign(s *Service, t time.Time) []byte {
	h := ghmac([]byte("AWS4"+a.GetSecretKey()), []byte(t.Format(iSO8601BasicFormatShort)))
	h = ghmac(h, []byte(s.Region))
	h = ghmac(h, []byte(s.Name))
	h = ghmac(h, []byte(AWS4_URL))
	return h
}

func retrieveAWSCredentials(role string) (map[string]string, error) {
	var bodybytes []byte
	// Retrieve the json for this role
	resp, err := http.Get(fmt.Sprintf("%s/%s", AWSIAMCredsURL, role))
	if err != nil || resp.StatusCode != http.StatusOK {
		return nil, err
	}
	defer resp.Body.Close()

	bodybytes, err = ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	jsondata := make(map[string]string)
	err = json.Unmarshal(bodybytes, &jsondata)
	if err != nil {
		return nil, err
	}

	return jsondata, nil
}

func retrieveIAMRole() (string, error) {
	var bodybytes []byte

	resp, err := http.Get(AWSIAMCredsURL)
	if err != nil || resp.StatusCode != http.StatusOK {
		return "", err
	}
	defer resp.Body.Close()

	bodybytes, err = ioutil.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	// pick the first IAM role
	role := strings.Split(string(bodybytes), "\n")[0]
	if len(role) == 0 {
		return "", errors.New("Unable to retrieve IAM role")
	}

	return role, nil
}
