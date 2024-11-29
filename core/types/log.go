// Copyright 2014 The go-ethereum Authors
// (original work)
// Copyright 2024 The Erigon Authors
// (modifications)
// This file is part of Erigon.
//
// Erigon is free software: you can redistribute it and/or modify
// it under the terms of the GNU Lesser General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// Erigon is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU Lesser General Public License for more details.
//
// You should have received a copy of the GNU Lesser General Public License
// along with Erigon. If not, see <http://www.gnu.org/licenses/>.

package types

import (
	"io"

	"github.com/erigontech/erigon-lib/common/hexutil"

	libcommon "github.com/erigontech/erigon-lib/common"
	"github.com/erigontech/erigon-lib/common/hexutility"

	"github.com/erigontech/erigon-lib/rlp"
)

//(go:generate gencodec -type Log -field-override logMarshaling -out gen_log_json.go)

// Log represents a contract log event. These events are generated by the LOG opcode and
// stored/indexed by the node.
type Log struct {
	// Consensus fields:
	// address of the contract that generated the event
	Address libcommon.Address `json:"address" gencodec:"required" codec:"1"`
	// list of topics provided by the contract.
	Topics []libcommon.Hash `json:"topics" gencodec:"required" codec:"2"`
	// supplied by the contract, usually ABI-encoded
	Data []byte `json:"data" gencodec:"required" codec:"3"`

	// Derived fields. These fields are filled in by the node
	// but not secured by consensus.
	// block in which the transaction was included
	BlockNumber uint64 `json:"blockNumber" codec:"-"`

	// hash of the transaction
	TxHash libcommon.Hash `json:"transactionHash" gencodec:"required" codec:"-"`
	// index of the transaction in the block
	TxIndex uint `json:"transactionIndex" codec:"-"`
	// hash of the block in which the transaction was included
	BlockHash libcommon.Hash `json:"blockHash" codec:"-"`
	// index of the log in the block
	Index uint `json:"logIndex" codec:"-"`

	// The Removed field is true if this log was reverted due to a chain reorganisation.
	// You must pay attention to this field if you receive logs through a filter query.
	Removed bool `json:"removed" codec:"-"`
}

type ErigonLog struct {
	Address     libcommon.Address `json:"address" gencodec:"required" codec:"1"`
	Topics      []libcommon.Hash  `json:"topics" gencodec:"required" codec:"2"`
	Data        []byte            `json:"data" gencodec:"required" codec:"3"`
	BlockNumber uint64            `json:"blockNumber" codec:"-"`
	TxHash      libcommon.Hash    `json:"transactionHash" gencodec:"required" codec:"-"`
	TxIndex     uint              `json:"transactionIndex" codec:"-"`
	BlockHash   libcommon.Hash    `json:"blockHash" codec:"-"`
	Index       uint              `json:"logIndex" codec:"-"`
	Removed     bool              `json:"removed" codec:"-"`
	Timestamp   uint64            `json:"timestamp" codec:"-"`
}

type ErigonLogs []*ErigonLog

type Logs []*Log

func (logs Logs) Filter(addrMap map[libcommon.Address]struct{}, topics [][]libcommon.Hash, maxLogs uint64) Logs {
	topicMap := make(map[int]map[libcommon.Hash]struct{}, 7)

	//populate topic map
	for idx, v := range topics {
		for _, vv := range v {
			if _, ok := topicMap[idx]; !ok {
				topicMap[idx] = map[libcommon.Hash]struct{}{}
			}
			topicMap[idx][vv] = struct{}{}
		}
	}

	o := make(Logs, 0, len(logs))
	var logCount uint64
	logCount = 0
	for _, v := range logs {
		// check address if addrMap is not empty
		if len(addrMap) != 0 {
			if _, ok := addrMap[v.Address]; !ok {
				// not there? skip this log
				continue
			}
		}

		// If the to filtered topics is greater than the amount of topics in logs, skip.
		if len(topics) > len(v.Topics) {
			continue
		}
		// the default state is to include the log
		found := true
		// if there are no topics provided, then match all
		for idx, topicSet := range topicMap {
			// if the topicSet is empty, match all as wildcard
			if len(topicSet) == 0 {
				continue
			}
			// the topicSet isnt empty, so the topic must be included.
			if _, ok := topicSet[v.Topics[idx]]; !ok {
				// the topic wasn't found, so we should skip this log
				found = false
				break
			}
		}
		if found {
			o = append(o, v)
		}

		logCount += 1
		if maxLogs != 0 && logCount >= maxLogs {
			break
		}
	}
	return o
}

func (logs Logs) CointainTopics(addrMap map[libcommon.Address]struct{}, topicsMap map[libcommon.Hash]struct{}, maxLogs uint64) Logs {
	o := make(Logs, 0, len(logs))
	var logCount uint64
	logCount = 0
	for _, v := range logs {
		found := false

		// check address if addrMap is not empty
		if len(addrMap) != 0 {
			if _, ok := addrMap[v.Address]; !ok {
				// not there? skip this log
				continue
			}
		}
		//topicsMap len zero match any topics
		if len(topicsMap) == 0 {
			o = append(o, v)
		} else {
			for i := range v.Topics {
				//Contain any topics that matched
				if _, ok := topicsMap[v.Topics[i]]; ok {
					found = true
				}
			}
			if found {
				o = append(o, v)
			}
		}
		logCount += 1
		if maxLogs != 0 && logCount >= maxLogs {
			break
		}
	}
	return o
}

func (logs Logs) FilterOld(addresses map[libcommon.Address]struct{}, topics [][]libcommon.Hash) Logs {
	result := make(Logs, 0, len(logs))
	// populate a set of addresses
Logs:
	for _, log := range logs {
		// empty address list means no filter
		if len(addresses) > 0 {
			// this is basically the includes function but done with a map
			if _, ok := addresses[log.Address]; !ok {
				continue
			}
		}
		// If the to filtered topics is greater than the amount of topics in logs, skip.
		if len(topics) > len(log.Topics) {
			continue
		}
		for i, sub := range topics {
			match := len(sub) == 0 // empty rule set == wildcard
			// iterate over the subtopics and look for any match.
			for _, topic := range sub {
				if log.Topics[i] == topic {
					match = true
					break
				}
			}
			// there was no match, so this log is invalid.
			if !match {
				continue Logs
			}
		}
		result = append(result, log)
	}
	return result
}

type logMarshaling struct {
	Data        hexutility.Bytes
	BlockNumber hexutil.Uint64
	TxIndex     hexutil.Uint
	Index       hexutil.Uint
}

type rlpLog struct {
	Address libcommon.Address
	Topics  []libcommon.Hash
	Data    []byte
}

// rlpStorageLog is the storage encoding of a log.
type rlpStorageLog rlpLog

// legacyRlpStorageLog is the previous storage encoding of a log including some redundant fields.
type legacyRlpStorageLog struct {
	Address libcommon.Address
	Topics  []libcommon.Hash
	Data    []byte
	//BlockNumber uint64
	//TxHash      libcommon.Hash
	//TxIndex     uint
	//BlockHash   libcommon.Hash
	//Index       uint
}

// EncodeRLP implements rlp.Encoder.
func (l *Log) EncodeRLP(w io.Writer) error {
	return rlp.Encode(w, rlpLog{Address: l.Address, Topics: l.Topics, Data: l.Data})
}

// DecodeRLP implements rlp.Decoder.
func (l *Log) DecodeRLP(s *rlp.Stream) error {
	var dec rlpLog
	err := s.Decode(&dec)
	if err == nil {
		l.Address, l.Topics, l.Data = dec.Address, dec.Topics, dec.Data
	}
	return err
}

// Copy creates a deep copy of the Log.
func (l *Log) Copy() *Log {
	topics := make([]libcommon.Hash, 0, len(l.Topics))
	for _, topic := range l.Topics {
		topicCopy := libcommon.BytesToHash(topic.Bytes())
		topics = append(topics, topicCopy)
	}

	data := make([]byte, len(l.Data))
	copy(data, l.Data)

	return &Log{
		Address:     libcommon.BytesToAddress(l.Address.Bytes()),
		Topics:      topics,
		Data:        data,
		BlockNumber: l.BlockNumber,
		TxHash:      libcommon.BytesToHash(l.TxHash.Bytes()),
		TxIndex:     l.TxIndex,
		BlockHash:   libcommon.BytesToHash(l.BlockHash.Bytes()),
		Index:       l.Index,
		Removed:     l.Removed,
	}
}

// LogForStorage is a wrapper around a Log that flattens and parses the entire content of
// a log including non-consensus fields.
type LogForStorage Log

// EncodeRLP implements rlp.Encoder.
func (l *LogForStorage) EncodeRLP(w io.Writer) error {
	return rlp.Encode(w, rlpStorageLog{
		Address: l.Address,
		Topics:  l.Topics,
		Data:    l.Data,
		//BlockNumber: l.BlockNumber,
		//TxHash:      l.TxHash,
		//TxIndex:     l.TxIndex,
		//BlockHash:   l.BlockHash,
		//Index:       l.Index,
	})
}

// DecodeRLP implements rlp.Decoder.
//
// Note some redundant fields(e.g. block number, txn hash etc) will be assembled later.
func (l *LogForStorage) DecodeRLP(s *rlp.Stream) error {
	blob, err := s.Raw()
	if err != nil {
		return err
	}
	var dec rlpStorageLog
	err = rlp.DecodeBytes(blob, &dec)
	if err == nil {
		*l = LogForStorage{
			Address: dec.Address,
			Topics:  dec.Topics,
			Data:    dec.Data,
		}
	} else {
		// Try to decode log with previous definition.
		var dec legacyRlpStorageLog
		err = rlp.DecodeBytes(blob, &dec)
		if err == nil {
			*l = LogForStorage{
				Address: dec.Address,
				Topics:  dec.Topics,
				Data:    dec.Data,
			}
		}
	}
	return err
}
