package lightning_api

import (
	"fmt"
	"math/big"
	"strconv"
	"strings"

	"github.com/golang/glog"
)

func ToLndChanId(id string) (uint64, error) {
	split := strings.Split(strings.ToLower(id), "x")
	if len(split) != 3 {
		return 0, fmt.Errorf("wrong channel id: %v", id)
	}

	blockId, err := strconv.ParseUint(split[0], 10, 64)
	if err != nil {
		return 0, err
	}

	txIdx, err := strconv.ParseUint(split[1], 10, 64)
	if err != nil {
		return 0, err
	}

	outputIdx, err := strconv.ParseUint(split[2], 10, 64)
	if err != nil {
		return 0, err
	}

	result := (blockId&0xffffff)<<40 | (txIdx&0xffffff)<<16 | (outputIdx & 0xffff)

	return result, nil
}

func FromLndChanId(chanId uint64) string {
	blockId := int64((chanId & 0xffffff0000000000) >> 40)
	txIdx := int((chanId & 0x000000ffffff0000) >> 16)
	outputIdx := int(chanId & 0x000000000000ffff)

	return fmt.Sprintf("%dx%dx%d", blockId, txIdx, outputIdx)
}

func ConvertAmount(s string) uint64 {
	x := strings.ReplaceAll(s, "msat", "")
	ret, err := strconv.ParseUint(x, 10, 64)
	if err != nil {
		glog.Warningf("Could not convert: %v %v", s, err)
		return 0
	}

	return ret
}

func ConvertFeatures(features string) map[string]NodeFeatureApi {
	n := new(big.Int)

	n, ok := n.SetString(features, 16)
	if !ok {
		return nil
	}

	result := make(map[string]NodeFeatureApi)

	m := big.NewInt(0)
	zero := big.NewInt(0)
	two := big.NewInt(2)

	bit := 0
	for n.Cmp(zero) == 1 {
		n.DivMod(n, two, m)

		if m.Cmp(zero) != 1 {
			// Bit is not set
			bit++
			continue
		}

		result[fmt.Sprintf("%d", bit)] = NodeFeatureApi{
			Name:       "",
			IsKnown:    true,
			IsRequired: bit%2 == 0,
		}

		bit++
	}

	return result
}

func SumCapacitySimple(channels []NodeChannelApi) uint64 {
	sum := uint64(0)
	for _, channel := range channels {
		sum += channel.Capacity
	}

	return sum
}

func SumCapacityExtended(channels []NodeChannelApiExtended) uint64 {
	sum := uint64(0)
	for _, channel := range channels {
		sum += channel.Capacity
	}

	return sum
}