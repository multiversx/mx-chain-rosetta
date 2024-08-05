// This is a one-file HTTP forwarder: captures requests destined to the MultiversX Observer API and transforms them into requests to the MultiversX Proxy API.
// It handles GET and POST requests used by the MultiversX Rosetta implementation, so that the Rosetta implementation can be tested against the MultiversX Proxy API
// (without it being aware).
package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/urfave/cli"
)

var (
	cliFlagProxyUrl = cli.StringFlag{
		Name:     "proxy",
		Required: true,
	}

	cliFlagShard = cli.UintFlag{
		Name:     "shard",
		Required: true,
	}

	cliFlagSleep = cli.UintFlag{
		Name:     "sleep",
		Required: true,
	}
)

func main() {
	app := cli.NewApp()
	app.Action = startAdapter
	app.Flags = []cli.Flag{
		cliFlagProxyUrl,
		cliFlagShard,
		cliFlagSleep,
	}

	err := app.Run(os.Args)
	if err != nil {
		log.Fatal(err.Error())
		os.Exit(1)
	}
}

func startAdapter(ctx *cli.Context) {
	adapter := adapter{
		proxyUrl:      ctx.GlobalString(cliFlagProxyUrl.Name),
		shard:         ctx.GlobalUint(cliFlagShard.Name),
		sleepDuration: ctx.GlobalUint(cliFlagSleep.Name),
	}

	router := gin.Default()
	router.GET("/node/status", adapter.getNodeStatus)
	router.GET("/node/epoch-start/:epoch", adapter.getEpochStart)
	router.GET("/block/by-nonce/:nonce", adapter.getBlockByNonce)
	router.GET("/address/:address/esdt/:token", adapter.getAccountESDT)
	router.GET("/address/:address", adapter.getAccount)
	router.POST("/transaction/send", adapter.sendTransaction)
	router.Run(":8080")
}

type adapter struct {
	shard         uint
	proxyUrl      string
	sleepDuration uint
}

func (adapter *adapter) getNodeStatus(c *gin.Context) {
	url := fmt.Sprintf("network/status/%d", adapter.shard)

	adapter.forwardGetRequest(c, url, func(response map[string]interface{}) {
		data := response["data"].(map[string]interface{})
		data["metrics"] = data["status"]
		data["metrics"].(map[string]interface{})["erd_app_version"] = "v1.2.3"
		data["metrics"].(map[string]interface{})["erd_public_key_block_sign"] = "00"
		delete(data, "status")
	})
}

func (adapter *adapter) getEpochStart(c *gin.Context) {
	epoch := c.Param("epoch")
	url := fmt.Sprintf("network/epoch-start/%d/by-epoch/%s", adapter.shard, epoch)
	adapter.forwardGetRequest(c, url, nil)
}

func (adapter *adapter) getBlockByNonce(c *gin.Context) {
	nonce := c.Param("nonce")
	url := fmt.Sprintf("block/%d/by-nonce/%s", adapter.shard, nonce)
	adapter.forwardGetRequest(c, url, nil)
}

func (adapter *adapter) getAccountESDT(c *gin.Context) {
	address := c.Param("address")
	token := c.Param("token")
	url := fmt.Sprintf("address/%s/esdt/%s", address, token)
	adapter.forwardGetRequest(c, url, nil)
}

func (adapter *adapter) getAccount(c *gin.Context) {
	address := c.Param("address")
	url := fmt.Sprintf("address/%s", address)
	adapter.forwardGetRequest(c, url, nil)
}

func (adapter *adapter) sendTransaction(c *gin.Context) {
	url := fmt.Sprintf("transaction/send")
	adapter.forwardPostRequest(c, url)
}

func (adapter *adapter) forwardGetRequest(c *gin.Context, urlPath string, mutateResponse func(map[string]interface{})) {
	// Delay GET requests, to avoid reaching the rate limit.
	adapter.sleep()

	urlObject := url.URL{
		Path:     urlPath,
		RawQuery: c.Request.URL.Query().Encode(),
	}

	url := fmt.Sprintf("%s/%s", adapter.proxyUrl, urlObject.String())

	rawResponse, err := http.Get(url)
	if err != nil {
		log.Printf("Error fetching %s: %v", url, err)
		adapter.emitInternalServerError(c, err)
		return
	}
	defer rawResponse.Body.Close()

	var response map[string]interface{}
	if err := json.NewDecoder(rawResponse.Body).Decode(&response); err != nil {
		log.Printf("Error decoding response: %v", err)
		adapter.emitInternalServerError(c, err)
		return
	}

	if mutateResponse != nil {
		mutateResponse(response)
	}

	c.JSON(rawResponse.StatusCode, response)
}

func (adapter *adapter) forwardPostRequest(c *gin.Context, urlPath string) {
	url := fmt.Sprintf("%s/%s", adapter.proxyUrl, urlPath)

	var data map[string]interface{}
	if err := c.BindJSON(&data); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Bad request"})
		return
	}

	jsonData, err := json.Marshal(data)
	if err != nil {
		adapter.emitInternalServerError(c, err)
		return
	}

	rawResponse, err := http.Post(url, "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		adapter.emitInternalServerError(c, err)
		return
	}
	defer rawResponse.Body.Close()

	var response map[string]interface{}
	if err := json.NewDecoder(rawResponse.Body).Decode(&response); err != nil {
		adapter.emitInternalServerError(c, err)
		return
	}

	c.JSON(rawResponse.StatusCode, response)
}

func (adapter *adapter) sleep() {
	time.Sleep(time.Duration(adapter.sleepDuration) * time.Millisecond)
}

func (adapter *adapter) emitInternalServerError(c *gin.Context, err error) {
	c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
}
