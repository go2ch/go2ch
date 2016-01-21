package go2ch

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"testing"
)

var c *Client

func setup() {
	var apiKey map[string]string
	keyFile, err := ioutil.ReadFile("./go2ch_test.json")
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	json.Unmarshal(keyFile, &apiKey)
	c = NewClient(apiKey["appkey"], apiKey["hmkey"])
}

func TestRequest(t *testing.T) {
	setup()

	headers := make(map[string]string)
	resp, err := c.Get("peace", "unix", "999935885", headers)

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

