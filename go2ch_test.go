package go2ch

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"testing"
)

var apiKey map[string]string
var c *Client

func TestMain(m *testing.M) {
	keyFile, err := ioutil.ReadFile("./go2ch_test.json")
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	json.Unmarshal(keyFile, &apiKey)
	c = NewClient(apiKey["appkey"], apiKey["hmkey"])

	os.Exit(m.Run())
}

func TestRequest(t *testing.T) {
	headers := make(map[string]string)
	resp, err := c.Get("echo", "unix", "999935885", headers)

	if err != nil {
		if err.Error() == "forbidden" {
			t.Skip("API access forbidden")
		}

		t.Fatalf("unexpected error: %s", err)
	}

	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		t.Fatalf("unexpected status code: %d", resp.StatusCode)
	}
}

