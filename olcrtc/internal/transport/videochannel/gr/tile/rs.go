package tile

import "math"

func rsShards(dataBytes, pctParity int) (data, parity int) {
	if pctParity == 0 {
		return 1, 0
	}
	data = max(1, dataBytes/256)
	data = min(data, 256-int(math.Ceil(float64(data)*float64(pctParity)/100)))
	if data < 1 {
		data = 1
	}
	parity = int(math.Ceil(float64(data) * float64(pctParity) / 100))
	if parity < 1 {
		parity = 1
	}
	for data+parity > 256 {
		data--
	}
	return data, parity
}

func shardBytes(totalBytes, dataShards int) int {
	return (totalBytes + dataShards - 1) / dataShards
}

func splitShards(data []byte, n, shardSz int) [][]byte {
	shards := make([][]byte, n)
	for i := range shards {
		start := i * shardSz
		end := start + shardSz
		shard := make([]byte, shardSz)
		if start < len(data) {
			if end > len(data) {
				end = len(data)
			}
			copy(shard, data[start:end])
		}
		shards[i] = shard
	}
	return shards
}

func joinShards(shards [][]byte) []byte {
	total := 0
	for _, s := range shards {
		total += len(s)
	}
	out := make([]byte, 0, total)
	for _, s := range shards {
		out = append(out, s...)
	}
	return out
}
