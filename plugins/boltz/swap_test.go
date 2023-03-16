//go:build plugins
// +build plugins

package boltz

import (
	"context"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"testing"
	"time"

	agent_entities "github.com/bolt-observer/agent/entities"
	filter "github.com/bolt-observer/agent/filter"
	api "github.com/bolt-observer/agent/lightning"
	common_entities "github.com/bolt-observer/go_common/entities"
	"github.com/stretchr/testify/assert"
	bip39 "github.com/tyler-smith/go-bip39"
)

const (
	LnRegTestPathPrefix = "/tmp/lnregtest-data/dev_network/"
	BoltzUrl            = "http://localhost:9001"
	TestnetBoltzUrl     = "https://testnet.boltz.exchange/api"
	Regtest             = "regtest"
	Tesnet              = "testnet"
)

// Make sure to increase test timeout to 60 s
// Visual Studio Code: Code / Settings / Settings / "go.TestTimeout" and change the 30s to 60s

// Useful commands:
// curl -X POST localhost:9001/swapstatus -d '{ "id": "9klCJS" }'  -H "Content-Type: application/json"
// {"status":"invoice.set"}
// bitcoin-cli -datadir=/tmp/lnregtest-data/dev_network/bitcoin -generate 1
// lncli --lnddir=/tmp/lnregtest-data/dev_network/lnnodes/A --rpcserver=localhost:11009 --macaroonpath=/tmp/lnregtest-data/dev_network/lnnodes/A/data/chain/bitcoin/regtest/admin.macaroon --network=regtest
// lightning-cli --lightning-dir=/tmp/lnregtest-data/dev_network/lnnodes/B --network=regtest
// lncli --lnddir=/tmp/lnregtest-data/dev_network/lnnodes/C --rpcserver=localhost:11011 --macaroonpath=/tmp/lnregtest-data/dev_network/lnnodes/C/data/chain/bitcoin/regtest/admin.macaroon --network=regtest
// lncli --lnddir=/tmp/lnregtest-data/dev_network/lnnodes/D --rpcserver=localhost:11012 --macaroonpath=/tmp/lnregtest-data/dev_network/lnnodes/D/data/chain/bitcoin/regtest/admin.macaroon --network=regtest
// lncli --lnddir=/tmp/lnregtest-data/dev_network/lnnodes/E --rpcserver=localhost:11013 --macaroonpath=/tmp/lnregtest-data/dev_network/lnnodes/E/data/chain/bitcoin/regtest/admin.macaroon --network=regtest
// lncli --lnddir=/tmp/lnregtest-data/dev_network/lnnodes/F --rpcserver=localhost:11014 --macaroonpath=/tmp/lnregtest-data/dev_network/lnnodes/F/data/chain/bitcoin/regtest/admin.macaroon --network=regtest
// lncli --lnddir=/tmp/lnregtest-data/dev_network/lnnodes/G --rpcserver=localhost:11015 --macaroonpath=/tmp/lnregtest-data/dev_network/lnnodes/G/data/chain/bitcoin/regtest/admin.macaroon --network=regtest

func getLocalCln(t *testing.T, name string) agent_entities.NewAPICall {
	data := &common_entities.Data{}
	x := int(api.ClnSocket)
	data.Endpoint = fmt.Sprintf("%s/lnnodes/%s/regtest/lightning-rpc", LnRegTestPathPrefix, name)
	if _, err := os.Stat(data.Endpoint); errors.Is(err, os.ErrNotExist) {
		return nil
	}
	data.ApiType = &x

	return func() (api.LightingAPICalls, error) {
		api, err := api.NewAPI(api.ClnSocket, func() (*common_entities.Data, error) {
			return data, nil
		})

		return api, err
	}
}

func getLocalLndByName(t *testing.T, name string) agent_entities.NewAPICall {
	nodes := map[string]string{
		"A": "localhost:11009",
		"C": "localhost:11011",
		"D": "localhost:11012",
		"E": "localhost:11013",
		"F": "localhost:11014",
		"G": "localhost:11015",
	}

	return getLocalLnd(t, name, nodes[name])
}

func getLocalLnd(t *testing.T, name string, endpoint string) agent_entities.NewAPICall {
	data := &common_entities.Data{}
	x := int(api.LndGrpc)
	data.Endpoint = endpoint
	data.ApiType = &x

	content, err := os.ReadFile(fmt.Sprintf("%s/lnnodes/%s/tls.cert", LnRegTestPathPrefix, name))
	if err != nil {
		return nil
	}
	data.CertificateBase64 = base64.StdEncoding.EncodeToString(content)

	macBytes, err := os.ReadFile(fmt.Sprintf("%s/lnnodes/%s/data/chain/bitcoin/regtest/admin.macaroon", LnRegTestPathPrefix, name))
	if err != nil {
		return nil
	}
	data.MacaroonHex = hex.EncodeToString(macBytes)

	return func() (api.LightingAPICalls, error) {
		api, err := api.NewAPI(api.LndGrpc, func() (*common_entities.Data, error) {
			return data, nil
		})

		return api, err
	}
}

func getTestnetLnd(t *testing.T) agent_entities.NewAPICall {
	name := "testnet.secret"
	data := &common_entities.Data{}

	if _, err := os.Stat(name); errors.Is(err, os.ErrNotExist) {
		return nil
	}

	content, err := os.ReadFile(name)
	if err != nil {
		t.Fatalf("Error when opening file: %v", err)
		return nil
	}

	err = json.Unmarshal(content, &data)
	if err != nil {
		t.Fatalf("Error during Unmarshal(): %v", err)
		return nil
	}

	return func() (api.LightingAPICalls, error) {
		api, err := api.NewAPI(api.LndGrpc, func() (*common_entities.Data, error) {
			return data, nil
		})

		return api, err
	}
}

func mine(numBlocks int) error {
	_, err := exec.Command("bitcoin-cli", fmt.Sprintf("-datadir=%s/bitcoin", LnRegTestPathPrefix), "-generate", fmt.Sprintf("%d", numBlocks)).Output()
	return err
}

type LogAggregator struct {
	LogLines []string
	T        *testing.T
	Failure  bool
}

func NewLogAggregator(t *testing.T) *LogAggregator {
	return &LogAggregator{
		LogLines: make([]string, 0),
		T:        t,
		Failure:  false,
	}
}

func (l *LogAggregator) Log(msg agent_entities.PluginMessage) error {
	l.T.Logf("[%s] %+v\n", time.Now().Format(time.StampMilli), msg)
	if msg.IsError {
		l.Failure = true
	}
	l.LogLines = append(l.LogLines, msg.Message)

	return nil
}

func (l *LogAggregator) WasFailure() bool {
	return l.Failure
}

func (l *LogAggregator) WasSuccess() bool {
	regex := regexp.MustCompile("Swap .* succeeded")
	for _, msg := range l.LogLines {
		if regex.MatchString(msg) {
			return true
		}
	}

	return false
}

func nodeSanityCheck(t *testing.T, ln agent_entities.NewAPICall, name string) {
	// Sanity check of node
	ctx := context.Background()
	lnAPI, err := ln()
	assert.NotNil(t, lnAPI)
	assert.NoError(t, err)
	info, err := lnAPI.GetInfo(ctx)
	assert.NoError(t, err)
	assert.Equal(t, name, info.Alias)

	funds, err := lnAPI.GetOnChainFunds(ctx)
	assert.NoError(t, err)
	assert.Greater(t, funds.ConfirmedBalance, int64(1_000_000))
}

func newPlugin(t *testing.T, ln agent_entities.NewAPICall, dbName string, boltzUrl string, network string) *Plugin {
	f, err := filter.NewAllowAllFilter()
	assert.NoError(t, err)

	p, err := NewPlugin(ln, f, getMockCliCtx(boltzUrl, dbName, network), nil)
	assert.NoError(t, err)
	_, err = p.BoltzAPI.GetNodes()
	assert.NoError(t, err)

	entropy, err := bip39.NewEntropy(SecretBitSize)
	assert.NoError(t, err)
	p.CryptoAPI.MasterSecret = entropy
	return p
}

func TestSwapCln(t *testing.T) {
	const Node = "B"

	ln := getLocalCln(t, Node)
	if ln == nil {
		fmt.Printf("Ignoring swap test since regtest network is not available\n")
		return
	}

	nodeSanityCheck(t, ln, Node)

	tempf, err := os.CreateTemp("", "tempdb-")
	assert.NoError(t, err)
	defer os.RemoveAll(tempf.Name())

	p := newPlugin(t, ln, tempf.Name(), BoltzUrl, Regtest)

	l := NewLogAggregator(t)
	err = p.Execute(1339, []byte(`{ "target": "InboundLiquidityNodePercent", "percentage": 90}`), l.Log)
	assert.NoError(t, err)

	for i := 0; i < 20; i++ {
		err = mine(1)
		if err != nil {
			fmt.Printf("Could not mine %v\n", err)
		}
		if l.WasSuccess() {
			break
		}
		time.Sleep(5 * time.Second)
	}

	t.Fail()
}

func TestSwapLnd(t *testing.T) {
	const Node = "F"

	ln := getLocalLndByName(t, Node)
	if ln == nil {
		fmt.Printf("Ignoring swap test since regtest network is not available\n")
		return
	}

	nodeSanityCheck(t, ln, Node)

	tempf, err := os.CreateTemp("", "tempdb-")
	assert.NoError(t, err)
	defer os.RemoveAll(tempf.Name())

	p := newPlugin(t, ln, tempf.Name(), BoltzUrl, Regtest)

	l := NewLogAggregator(t)
	err = p.Execute(1339, []byte(`{ "target": "InboundLiquidityNodePercent", "percentage": 90}`), l.Log)
	assert.NoError(t, err)

	for i := 0; i < 5; i++ {
		err = mine(1)
		if err != nil {
			fmt.Printf("Could not mine %v", err)
		}
		if l.WasSuccess() {
			break
		}
		time.Sleep(5 * time.Second)
	}

	t.Fail()
}

func TestStateMachineRecovery(t *testing.T) {
	const Node = "F"

	ln := getLocalLndByName(t, Node)
	if ln == nil {
		fmt.Printf("Ignoring swap test since regtest network is not available\n")
		return
	}

	tempf, err := os.CreateTemp("", "tempdb-")
	assert.NoError(t, err)
	defer os.RemoveAll(tempf.Name())

	db := &BoltzDB{}
	err = db.Connect(tempf.Name())
	assert.NoError(t, err)

	sd := &SwapData{
		JobID: JobID(1336),
		Sats:  100000,
		State: SwapSuccess,
	}

	if err = db.Get(int32(sd.JobID), &sd); err != nil {
		err = db.Insert(int32(sd.JobID), sd)
	} else {
		err = db.Update(int32(sd.JobID), sd)
	}
	assert.NoError(t, err)
	db.db.Close()

	p := newPlugin(t, ln, tempf.Name(), BoltzUrl, Regtest)

	l := NewLogAggregator(t)
	err = p.Execute(1336, []byte(``), l.Log)
	assert.NoError(t, err)

	for i := 0; i < 10; i++ {
		if l.WasSuccess() {
			return
		}
		time.Sleep(1 * time.Second)
	}

	t.Fatalf("Did not get success in log")
}

func TestInboundTestnet(t *testing.T) {
	// go test -test.v -timeout 1h -tags plugins -run ^TestInboundTestnet$ github.com/bolt-observer/agent/plugins/boltz
	ln := getTestnetLnd(t)
	if ln == nil {
		fmt.Printf("Ignoring swap test since test network is not available\n")
		return
	}
	ctx := context.Background()
	lnAPI, err := ln()
	assert.NotNil(t, lnAPI)
	assert.NoError(t, err)
	info, err := lnAPI.GetInfo(ctx)
	assert.NoError(t, err)
	t.Logf("Info %v\n", info)

	tempf, err := os.CreateTemp("", "tempdb-")
	assert.NoError(t, err)
	defer os.RemoveAll(tempf.Name())

	p := newPlugin(t, ln, tempf.Name(), TestnetBoltzUrl, Tesnet)

	l := NewLogAggregator(t)

	err = p.Execute(1, []byte(`{ "target": "InboundLiquidityNodePercent", "percentage": 10}`), l.Log)
	assert.NoError(t, err)

	for i := 0; i < 360; i++ {
		if l.WasSuccess() {
			break
		}
		if l.WasFailure() {
			t.Fail()
			return
		}
		time.Sleep(10 * time.Second)
	}

	t.Logf("timed out")
	t.Fail()
}

func TestPayInvoice(t *testing.T) {
	return

	ctx := context.Background()

	lnA := getLocalLndByName(t, "A")
	lnC := getLocalLndByName(t, "C")

	lnAPI1, err := lnA()
	assert.NotNil(t, lnAPI1)
	assert.NoError(t, err)

	lnAPI2, err := lnC()
	assert.NotNil(t, lnAPI2)
	assert.NoError(t, err)

	_, err = lnAPI1.GetInfo(ctx)
	assert.NoError(t, err)
	_, err = lnAPI2.GetInfo(ctx)
	assert.NoError(t, err)

	invoice, err := lnAPI2.CreateInvoice(ctx, 3000000, "", "", 24*time.Hour)
	assert.NoError(t, err)

	t.Logf("Invoice %v\n", invoice.PaymentRequest)

	// 128642860515328
	resp, err := lnAPI1.PayInvoice(ctx, invoice.PaymentRequest, 0, []uint64{1337, 128642860515328, 1338})
	assert.NoError(t, err)
	t.Logf("Burek %v", resp)

	t.Fail()
}