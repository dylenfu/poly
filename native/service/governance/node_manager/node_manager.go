/*
 * Copyright (C) 2018 The ontology Authors
 * This file is part of The ontology library.
 *
 * The ontology is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Lesser General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * The ontology is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Lesser General Public License for more details.
 *
 * You should have received a copy of the GNU Lesser General Public License
 * along with The ontology.  If not, see <http://www.gnu.org/licenses/>.
 */

package node_manager

import (
	"bytes"
	"encoding/hex"
	"fmt"

	"github.com/ontio/multi-chain/common"
	"github.com/ontio/multi-chain/common/config"
	"github.com/ontio/multi-chain/core/genesis"
	cstates "github.com/ontio/multi-chain/core/states"
	"github.com/ontio/multi-chain/core/types"
	"github.com/ontio/multi-chain/native"
	"github.com/ontio/multi-chain/native/service/utils"
)

const (
	//status
	CandidateStatus Status = iota
	ConsensusStatus
	QuitingStatus
	BlackStatus

	//function name
	REGISTER_CANDIDATE   = "registerCandidate"
	UNREGISTER_CANDIDATE = "unRegisterCandidate"
	APPROVE_CANDIDATE    = "approveCandidate"
	REJECT_CANDIDATE     = "rejectCandidate"
	BLACK_NODE           = "blackNode"
	WHITE_NODE           = "whiteNode"
	QUIT_NODE            = "quitNode"
	UPDATE_CONFIG        = "updateConfig"
	UPDATE_GLOBAL_PARAM  = "updateGlobalParam"
	ADD_POS              = "addPos"
	REDUCE_POS           = "reducePos"
	COMMIT_DPOS          = "commitDpos"

	//key prefix
	GOVERNANCE_VIEW = "governanceView"
	VBFT_CONFIG     = "vbftConfig"
	PRE_CONFIG      = "preConfig"
	CANDIDITE_INDEX = "candidateIndex"
	PEER_APPLY      = "peerApply"
	PEER_POOL       = "peerPool"
	PEER_INDEX      = "peerIndex"
	BLACK_LIST      = "blackList"
	GLOBAL_PARAM    = "globalParam"
)

//Register methods of node_manager contract
func RegisterNodeManagerContract(native *native.NativeService) {
	native.Register(genesis.INIT_CONFIG, InitConfig)
	native.Register(REGISTER_CANDIDATE, RegisterCandidate)
	native.Register(UNREGISTER_CANDIDATE, UnRegisterCandidate)
	native.Register(QUIT_NODE, QuitNode)
	native.Register(APPROVE_CANDIDATE, ApproveCandidate)
	native.Register(REJECT_CANDIDATE, RejectCandidate)
	native.Register(BLACK_NODE, BlackNode)
	native.Register(WHITE_NODE, WhiteNode)
	native.Register(UPDATE_CONFIG, UpdateConfig)
	native.Register(UPDATE_GLOBAL_PARAM, UpdateGlobalParam)
	native.Register(ADD_POS, AddPos)
	native.Register(REDUCE_POS, ReducePos)
	native.Register(COMMIT_DPOS, CommitDpos)
}

//Init node_manager contract
func InitConfig(native *native.NativeService) ([]byte, error) {
	configuration := new(config.VBFTConfig)
	if err := configuration.Deserialization(common.NewZeroCopySource(native.GetInput())); err != nil {
		return utils.BYTE_FALSE, fmt.Errorf("initConfig, contract params deserialize error: %v", err)
	}
	contract := utils.NodeManagerContractAddress

	// check if initConfig is already execute
	peerPoolMapBytes, err := native.GetCacheDB().Get(utils.ConcatKey(contract, []byte(PEER_POOL)))
	if err != nil {
		return utils.BYTE_FALSE, fmt.Errorf("initConfig, get peerPoolMap error: %v", err)
	}
	if peerPoolMapBytes != nil {
		return utils.BYTE_FALSE, fmt.Errorf("initConfig. initConfig is already executed")
	}

	//check the configuration
	err = CheckVBFTConfig(configuration)
	if err != nil {
		return utils.BYTE_FALSE, fmt.Errorf("initConfig, checkVBFTConfig failed: %v", err)
	}

	//init globalParam
	globalParam := &GlobalParam{
		MinInitStake: configuration.MinInitStake,
		CandidateNum: 7 * 7,
	}
	err = putGlobalParam(native, globalParam)
	if err != nil {
		return utils.BYTE_FALSE, fmt.Errorf("putGlobalParam, put globalParam error: %v", err)
	}

	var view uint32 = 1
	var maxId uint32

	peerPoolMap := &PeerPoolMap{
		PeerPoolMap: make(map[string]*PeerPoolItem),
	}
	for _, peer := range configuration.Peers {
		if peer.Index > maxId {
			maxId = peer.Index
		}
		address, err := common.AddressFromBase58(peer.Address)
		if err != nil {
			return utils.BYTE_FALSE, fmt.Errorf("initConfig, address format error: %v", err)
		}

		peerPoolItem := new(PeerPoolItem)
		peerPoolItem.Index = peer.Index
		peerPoolItem.PeerPubkey = peer.PeerPubkey
		peerPoolItem.Address = address[:]
		peerPoolItem.Status = ConsensusStatus
		peerPoolItem.Pos = peer.Pos
		peerPoolMap.PeerPoolMap[peerPoolItem.PeerPubkey] = peerPoolItem

		peerPubkeyPrefix, err := hex.DecodeString(peerPoolItem.PeerPubkey)
		if err != nil {
			return utils.BYTE_FALSE, fmt.Errorf("initConfig, peerPubkey format error: %v", err)
		}
		index := peerPoolItem.Index
		indexBytes := utils.GetUint32Bytes(index)
		native.GetCacheDB().Put(utils.ConcatKey(contract, []byte(PEER_INDEX), peerPubkeyPrefix), cstates.GenRawStorageItem(indexBytes))
	}

	//init peer pool
	err = putPeerPoolMap(native, peerPoolMap, 0)
	if err != nil {
		return utils.BYTE_FALSE, fmt.Errorf("initConfig, put peerPoolMap error: %v", err)
	}
	err = putPeerPoolMap(native, peerPoolMap, view)
	if err != nil {
		return utils.BYTE_FALSE, fmt.Errorf("initConfig, put peerPoolMap error: %v", err)
	}
	indexBytes := utils.GetUint32Bytes(maxId + 1)
	native.GetCacheDB().Put(utils.ConcatKey(contract, []byte(CANDIDITE_INDEX)), cstates.GenRawStorageItem(indexBytes))

	//init governance view
	governanceView := &GovernanceView{
		View:   view,
		Height: native.GetHeight(),
		TxHash: native.GetTx().Hash(),
	}
	err = putGovernanceView(native, governanceView)
	if err != nil {
		return utils.BYTE_FALSE, fmt.Errorf("putGovernanceView, put governanceView error: %v", err)
	}

	//init config
	config := &Configuration{
		N:                    configuration.N,
		C:                    configuration.C,
		K:                    configuration.K,
		L:                    configuration.L,
		BlockMsgDelay:        configuration.BlockMsgDelay,
		HashMsgDelay:         configuration.HashMsgDelay,
		PeerHandshakeTimeout: configuration.PeerHandshakeTimeout,
		MaxBlockChangeView:   configuration.MaxBlockChangeView,
	}
	err = putConfig(native, config)
	if err != nil {
		return utils.BYTE_FALSE, fmt.Errorf("initConfig, put config error: %v", err)
	}

	return utils.BYTE_TRUE, nil
}

//Register a candidate node, used by users.
func RegisterCandidate(native *native.NativeService) ([]byte, error) {
	params := new(RegisterPeerParam)
	if err := params.Deserialization(common.NewZeroCopySource(native.GetInput())); err != nil {
		return utils.BYTE_FALSE, fmt.Errorf("registerCandidate, contract params deserialize error: %v", err)
	}
	contract := utils.NodeManagerContractAddress

	address, err := common.AddressParseFromBytes(params.Address)
	if err != nil {
		return utils.BYTE_FALSE, fmt.Errorf("registerCandidate, common.AddressParseFromBytes error: %v", err)
	}
	//check witness
	err = utils.ValidateOwner(native, address)
	if err != nil {
		return utils.BYTE_FALSE, fmt.Errorf("registerCandidate, checkWitness error: %v", err)
	}

	//check peerPubkey
	if err := utils.ValidatePeerPubKeyFormat(params.PeerPubkey); err != nil {
		return utils.BYTE_FALSE, fmt.Errorf("registerCandidate, invalid peer pubkey")
	}

	peerPubkeyPrefix, err := hex.DecodeString(params.PeerPubkey)
	if err != nil {
		return utils.BYTE_FALSE, fmt.Errorf("registerCandidate, peerPubkey format error: %v", err)
	}
	//get black list
	blackList, err := native.GetCacheDB().Get(utils.ConcatKey(contract, []byte(BLACK_LIST), peerPubkeyPrefix))
	if err != nil {
		return utils.BYTE_FALSE, fmt.Errorf("registerCandidate, get BlackList error: %v", err)
	}
	if blackList != nil {
		return utils.BYTE_FALSE, fmt.Errorf("registerCandidate, this Peer is in BlackList")
	}

	//check if applied
	peer, err := GetPeeApply(native, params.PeerPubkey)
	if err != nil {
		return utils.BYTE_FALSE, fmt.Errorf("registerCandidate, GetPeeApply error: %v", err)
	}
	if peer != nil {
		return utils.BYTE_FALSE, fmt.Errorf("registerCandidate, peer already applied")
	}

	//get current view
	view, err := GetView(native)
	if err != nil {
		return utils.BYTE_FALSE, fmt.Errorf("blackNode, get view error: %v", err)
	}
	//get peerPoolMap
	peerPoolMap, err := GetPeerPoolMap(native, view)
	if err != nil {
		return utils.BYTE_FALSE, fmt.Errorf("registerCandidate, get peerPoolMap error: %v", err)
	}
	//check if exist in PeerPool
	_, ok := peerPoolMap.PeerPoolMap[params.PeerPubkey]
	if ok {
		return utils.BYTE_FALSE, fmt.Errorf("registerCandidate, peerPubkey is already in peerPoolMap")
	}

	err = putPeerApply(native, params)
	if err != nil {
		return utils.BYTE_FALSE, fmt.Errorf("registerCandidate, put putPeerApply error: %v", err)
	}

	//ont transfer
	err = appCallTransferOnt(native, address, utils.NodeManagerContractAddress, params.Pos)
	if err != nil {
		return utils.BYTE_FALSE, fmt.Errorf("registerCandidate, ont transfer error: %v", err)
	}

	return utils.BYTE_TRUE, nil
}

//Unregister a registered candidate node, will remove node from pool
func UnRegisterCandidate(native *native.NativeService) ([]byte, error) {
	params := new(PeerParam2)
	if err := params.Deserialization(common.NewZeroCopySource(native.GetInput())); err != nil {
		return utils.BYTE_FALSE, fmt.Errorf("unRegisterCandidate, contract params deserialize error: %v", err)
	}
	address, err := common.AddressParseFromBytes(params.Address)
	if err != nil {
		return utils.BYTE_FALSE, fmt.Errorf("unRegisterCandidate, common.AddressParseFromBytes error: %v", err)
	}
	contract := utils.NodeManagerContractAddress

	//check witness
	err = utils.ValidateOwner(native, address)
	if err != nil {
		return utils.BYTE_FALSE, fmt.Errorf("unRegisterCandidate, checkWitness error: %v", err)
	}

	//check if applied
	peer, err := GetPeeApply(native, params.PeerPubkey)
	if err != nil {
		return utils.BYTE_FALSE, fmt.Errorf("unRegisterCandidate, GetPeeApply error: %v", err)
	}
	if peer == nil {
		return utils.BYTE_FALSE, fmt.Errorf("unRegisterCandidate, peer is not applied")
	}
	//check owner address
	if !bytes.Equal(peer.Address, params.Address) {
		return utils.BYTE_FALSE, fmt.Errorf("unRegisterCandidate, address is not peer owner")
	}

	peerPubkeyPrefix, err := hex.DecodeString(params.PeerPubkey)
	if err != nil {
		return utils.BYTE_FALSE, fmt.Errorf("unRegisterCandidate, peerPubkey format error: %v", err)
	}
	native.GetCacheDB().Delete(utils.ConcatKey(contract, []byte(PEER_APPLY), peerPubkeyPrefix))

	//ont transfer
	err = appCallTransferOnt(native, utils.NodeManagerContractAddress, address, peer.Pos)
	if err != nil {
		return utils.BYTE_FALSE, fmt.Errorf("appCallTransferOnt, ont transfer error: %v", err)
	}

	return utils.BYTE_TRUE, nil
}

//Approve a registered candidate node
func ApproveCandidate(native *native.NativeService) ([]byte, error) {
	params := new(PeerParam)
	if err := params.Deserialization(common.NewZeroCopySource(native.GetInput())); err != nil {
		return utils.BYTE_FALSE, fmt.Errorf("approveCandidate, contract params deserialize error: %v", err)
	}

	// get operator from database
	operatorAddress, err := types.AddressFromBookkeepers(genesis.GenesisBookkeepers)
	if err != nil {
		return utils.BYTE_FALSE, err
	}

	//check witness
	err = utils.ValidateOwner(native, operatorAddress)
	if err != nil {
		return utils.BYTE_FALSE, fmt.Errorf("approveCandidate, checkWitness error: %v", err)
	}
	contract := utils.NodeManagerContractAddress

	//check if applied
	peer, err := GetPeeApply(native, params.PeerPubkey)
	if err != nil {
		return utils.BYTE_FALSE, fmt.Errorf("approveCandidate, GetPeeApply error: %v", err)
	}
	if peer == nil {
		return utils.BYTE_FALSE, fmt.Errorf("approveCandidate, peer is not applied")
	}

	peerPoolItem := &PeerPoolItem{
		PeerPubkey: peer.PeerPubkey,
		Address:    peer.Address,
		Pos:        peer.Pos,
	}

	//get globalParam
	globalParam, err := getGlobalParam(native)
	if err != nil {
		return utils.BYTE_FALSE, fmt.Errorf("approveCandidate, getGlobalParam error: %v", err)
	}
	//check initPos
	if peerPoolItem.Pos < uint64(globalParam.MinInitStake) {
		return utils.BYTE_FALSE, fmt.Errorf("approveCandidate, init Pos must >= %v", globalParam.MinInitStake)
	}

	//check if has index
	peerPubkeyPrefix, err := hex.DecodeString(peer.PeerPubkey)
	if err != nil {
		return utils.BYTE_FALSE, fmt.Errorf("approveCandidate, peerPubkey format error: %v", err)
	}
	indexBytes, err := native.GetCacheDB().Get(utils.ConcatKey(contract, []byte(PEER_INDEX), peerPubkeyPrefix))
	if err != nil {
		return utils.BYTE_FALSE, fmt.Errorf("approveCandidate, get indexBytes error: %v", err)
	}
	if indexBytes != nil {
		value, err := cstates.GetValueFromRawStorageItem(indexBytes)
		if err != nil {
			return nil, fmt.Errorf("approveCandidate, get value from raw storage item error:%v", err)
		}
		index := utils.GetBytesUint32(value)
		peerPoolItem.Index = index
	} else {
		//get candidate index
		candidateIndex, err := getCandidateIndex(native)
		if err != nil {
			return nil, fmt.Errorf("approveCandidate, get candidateIndex error: %v", err)
		}
		peerPoolItem.Index = candidateIndex

		//update candidateIndex
		newCandidateIndex := candidateIndex + 1
		err = putCandidateIndex(native, newCandidateIndex)
		if err != nil {
			return nil, fmt.Errorf("approveCandidate, put candidateIndex error: %v", err)
		}

		indexBytes := utils.GetUint32Bytes(peerPoolItem.Index)
		native.GetCacheDB().Put(utils.ConcatKey(contract, []byte(PEER_INDEX), peerPubkeyPrefix), cstates.GenRawStorageItem(indexBytes))
	}

	//get current view
	view, err := GetView(native)
	if err != nil {
		return utils.BYTE_FALSE, fmt.Errorf("approveCandidate, get view error: %v", err)
	}
	//get peerPoolMap
	peerPoolMap, err := GetPeerPoolMap(native, view)
	if err != nil {
		return utils.BYTE_FALSE, fmt.Errorf("approveCandidate, get peerPoolMap error: %v", err)
	}

	num := 0
	for _, peerPoolItem := range peerPoolMap.PeerPoolMap {
		if peerPoolItem.Status == CandidateStatus || peerPoolItem.Status == ConsensusStatus {
			num = num + 1
		}
	}
	if num >= int(globalParam.CandidateNum) {
		return utils.BYTE_FALSE, fmt.Errorf("approveCandidate, num of candidate node is full")
	}

	peerPoolItem.Status = CandidateStatus
	peerPoolMap.PeerPoolMap[params.PeerPubkey] = peerPoolItem
	err = putPeerPoolMap(native, peerPoolMap, view)
	if err != nil {
		return utils.BYTE_FALSE, fmt.Errorf("approveCandidate, put peerPoolMap error: %v", err)
	}

	return utils.BYTE_TRUE, nil
}

//Reject a registered candidate node, remove node from pool
func RejectCandidate(native *native.NativeService) ([]byte, error) {
	params := new(PeerParam)
	if err := params.Deserialization(common.NewZeroCopySource(native.GetInput())); err != nil {
		return utils.BYTE_FALSE, fmt.Errorf("rejectCandidate, contract params deserialize error: %v", err)
	}
	contract := utils.NodeManagerContractAddress

	// get operator from database
	operatorAddress, err := types.AddressFromBookkeepers(genesis.GenesisBookkeepers)
	if err != nil {
		return utils.BYTE_FALSE, err
	}

	//check witness
	err = utils.ValidateOwner(native, operatorAddress)
	if err != nil {
		return utils.BYTE_FALSE, fmt.Errorf("rejectCandidate, checkWitness error: %v", err)
	}

	//check if applied
	peer, err := GetPeeApply(native, params.PeerPubkey)
	if err != nil {
		return utils.BYTE_FALSE, fmt.Errorf("rejectCandidate, GetPeeApply error: %v", err)
	}
	if peer == nil {
		return utils.BYTE_FALSE, fmt.Errorf("rejectCandidate, peer is not applied")
	}

	peerPubkeyPrefix, err := hex.DecodeString(params.PeerPubkey)
	if err != nil {
		return utils.BYTE_FALSE, fmt.Errorf("rejectCandidate, peerPubkey format error: %v", err)
	}
	native.GetCacheDB().Delete(utils.ConcatKey(contract, []byte(PEER_APPLY), peerPubkeyPrefix))

	address, err := common.AddressParseFromBytes(peer.Address)
	if err != nil {
		return utils.BYTE_FALSE, fmt.Errorf("rejectCandidate, common.AddressParseFromBytes error: %v", err)
	}
	//ont transfer
	err = appCallTransferOnt(native, utils.NodeManagerContractAddress, address, peer.Pos)
	if err != nil {
		return utils.BYTE_FALSE, fmt.Errorf("appCallTransferOnt, ont transfer error: %v", err)
	}

	return utils.BYTE_TRUE, nil
}

//Put a node into black list, remove node from pool
//Node in black list can't be registered.
func BlackNode(native *native.NativeService) ([]byte, error) {
	params := new(PeerListParam)
	if err := params.Deserialization(common.NewZeroCopySource(native.GetInput())); err != nil {
		return utils.BYTE_FALSE, fmt.Errorf("blackNode, contract params deserialize error: %v", err)
	}

	// get operator from database
	operatorAddress, err := types.AddressFromBookkeepers(genesis.GenesisBookkeepers)
	if err != nil {
		return utils.BYTE_FALSE, err
	}

	//check witness
	err = utils.ValidateOwner(native, operatorAddress)
	if err != nil {
		return utils.BYTE_FALSE, fmt.Errorf("blackNode, checkWitness error: %v", err)
	}
	contract := utils.NodeManagerContractAddress

	//get current view
	view, err := GetView(native)
	if err != nil {
		return utils.BYTE_FALSE, fmt.Errorf("blackNode, get view error: %v", err)
	}
	//get peerPoolMap
	peerPoolMap, err := GetPeerPoolMap(native, view)
	if err != nil {
		return utils.BYTE_FALSE, fmt.Errorf("blackNode, get peerPoolMap error: %v", err)
	}

	commit := false
	for _, peerPubkey := range params.PeerPubkeyList {
		peerPubkeyPrefix, err := hex.DecodeString(peerPubkey)
		if err != nil {
			return utils.BYTE_FALSE, fmt.Errorf("blackNode, peerPubkey format error: %v", err)
		}
		peerPoolItem, ok := peerPoolMap.PeerPoolMap[peerPubkey]
		if !ok {
			return utils.BYTE_FALSE, fmt.Errorf("blackNode, peerPubkey is not in peerPoolMap")
		}

		blackListItem := &BlackListItem{
			PeerPubkey: peerPoolItem.PeerPubkey,
			Address:    peerPoolItem.Address,
			Pos:        peerPoolItem.Pos,
		}
		sink := common.NewZeroCopySink(nil)
		blackListItem.Serialization(sink)
		//put peer into black list
		native.GetCacheDB().Put(utils.ConcatKey(contract, []byte(BLACK_LIST), peerPubkeyPrefix), cstates.GenRawStorageItem(sink.Bytes()))

		//change peerPool status
		if peerPoolItem.Status == ConsensusStatus {
			commit = true
		}
		peerPoolItem.Status = BlackStatus
		peerPoolMap.PeerPoolMap[peerPubkey] = peerPoolItem
	}
	err = putPeerPoolMap(native, peerPoolMap, view)
	if err != nil {
		return utils.BYTE_FALSE, fmt.Errorf("blackNode, put peerPoolMap error: %v", err)
	}

	//commitDpos
	if commit {
		err = executeCommitDpos(native)
		if err != nil {
			return utils.BYTE_FALSE, fmt.Errorf("blackNode, executeCommitDpos error: %v", err)
		}
	}
	return utils.BYTE_TRUE, nil
}

//Remove a node from black list, allow it to be registered
func WhiteNode(native *native.NativeService) ([]byte, error) {
	params := new(PeerParam)
	if err := params.Deserialization(common.NewZeroCopySource(native.GetInput())); err != nil {
		return utils.BYTE_FALSE, fmt.Errorf("whiteNode, contract params deserialize error: %v", err)
	}

	// get operator from database
	operatorAddress, err := types.AddressFromBookkeepers(genesis.GenesisBookkeepers)
	if err != nil {
		return utils.BYTE_FALSE, err
	}

	//check witness
	err = utils.ValidateOwner(native, operatorAddress)
	if err != nil {
		return utils.BYTE_FALSE, fmt.Errorf("whiteNode, checkWitness error: %v", err)
	}
	contract := utils.NodeManagerContractAddress

	peerPubkeyPrefix, err := hex.DecodeString(params.PeerPubkey)
	if err != nil {
		return utils.BYTE_FALSE, fmt.Errorf("whiteNode, peerPubkey format error: %v", err)
	}

	//check black list
	blackListBytes, err := native.GetCacheDB().Get(utils.ConcatKey(contract, []byte(BLACK_LIST), peerPubkeyPrefix))
	if err != nil {
		return utils.BYTE_FALSE, fmt.Errorf("whiteNode, get BlackList error: %v", err)
	}
	if blackListBytes == nil {
		return utils.BYTE_FALSE, fmt.Errorf("whiteNode, this Peer is not in BlackList: %v", err)
	}

	//remove peer from black list
	native.GetCacheDB().Delete(utils.ConcatKey(contract, []byte(BLACK_LIST), peerPubkeyPrefix))

	return utils.BYTE_TRUE, nil
}

//Quit a registered node, used by node owner.
//Remove node from pool
func QuitNode(native *native.NativeService) ([]byte, error) {
	params := new(PeerParam2)
	if err := params.Deserialization(common.NewZeroCopySource(native.GetInput())); err != nil {
		return utils.BYTE_FALSE, fmt.Errorf("quitNode, contract params deserialize error: %v", err)
	}
	address, err := common.AddressParseFromBytes(params.Address)
	if err != nil {
		return utils.BYTE_FALSE, fmt.Errorf("quitNode, common.AddressParseFromBytes error: %v", err)
	}

	//check witness
	err = utils.ValidateOwner(native, address)
	if err != nil {
		return utils.BYTE_FALSE, fmt.Errorf("quitNode, checkWitness error: %v", err)
	}

	//get current view
	view, err := GetView(native)
	if err != nil {
		return utils.BYTE_FALSE, fmt.Errorf("quitNode, get view error: %v", err)
	}
	//get peerPoolMap
	peerPoolMap, err := GetPeerPoolMap(native, view)
	if err != nil {
		return utils.BYTE_FALSE, fmt.Errorf("quitNode, get peerPoolMap error: %v", err)
	}

	peerPoolItem, ok := peerPoolMap.PeerPoolMap[params.PeerPubkey]
	if !ok {
		return utils.BYTE_FALSE, fmt.Errorf("quitNode, peerPubkey is not in peerPoolMap")
	}
	if peerPoolItem.Status != ConsensusStatus && peerPoolItem.Status != CandidateStatus {
		return utils.BYTE_FALSE, fmt.Errorf("quitNode, peerPubkey is not CandidateStatus or ConsensusStatus")
	}
	if !bytes.Equal(params.Address, peerPoolItem.Address) {
		return utils.BYTE_FALSE, fmt.Errorf("quitNode, peerPubkey is not registered by this address")
	}

	//get config
	config, err := GetConfig(native)
	if err != nil {
		return utils.BYTE_FALSE, fmt.Errorf("quitNode, get config error: %v", err)
	}
	preConfig, err := getPreConfig(native)
	if err != nil {
		return utils.BYTE_FALSE, fmt.Errorf("quitNode, get preConfig error: %v", err)
	}
	if preConfig.SetView == view {
		config = preConfig.Configuration
	}

	//check peers num
	num := 0
	for _, peerPoolItem := range peerPoolMap.PeerPoolMap {
		if peerPoolItem.Status == CandidateStatus || peerPoolItem.Status == ConsensusStatus {
			num = num + 1
		}
	}
	if num <= int(config.K) {
		return utils.BYTE_FALSE, fmt.Errorf("quitNode, num of peers is less than K")
	}

	//change peerPool status
	peerPoolItem.Status = QuitingStatus

	peerPoolMap.PeerPoolMap[params.PeerPubkey] = peerPoolItem
	err = putPeerPoolMap(native, peerPoolMap, view)
	if err != nil {
		return utils.BYTE_FALSE, fmt.Errorf("quitNode, put peerPoolMap error: %v", err)
	}

	return utils.BYTE_TRUE, nil
}

//Go to next consensus epoch
func CommitDpos(native *native.NativeService) ([]byte, error) {
	// get config
	config, err := GetConfig(native)
	if err != nil {
		return utils.BYTE_FALSE, fmt.Errorf("commitDpos, get config error: %v", err)
	}

	//get governace view
	governanceView, err := GetGovernanceView(native)
	if err != nil {
		return utils.BYTE_FALSE, fmt.Errorf("commitDpos, get GovernanceView error: %v", err)
	}

	// get operator from database
	operatorAddress, err := types.AddressFromBookkeepers(genesis.GenesisBookkeepers)
	if err != nil {
		return utils.BYTE_FALSE, err
	}

	//check witness
	err = utils.ValidateOwner(native, operatorAddress)
	if err != nil {
		cycle := (native.GetHeight() - governanceView.Height) >= config.MaxBlockChangeView
		if !cycle {
			return utils.BYTE_FALSE, fmt.Errorf("commitDpos, authentication Failed")
		}
	}

	err = executeCommitDpos(native)
	if err != nil {
		return utils.BYTE_FALSE, fmt.Errorf("executeCommitDpos, executeCommitDpos error: %v", err)
	}

	return utils.BYTE_TRUE, nil
}

//Update VBFT config
func UpdateConfig(native *native.NativeService) ([]byte, error) {
	// get operator from database
	operatorAddress, err := types.AddressFromBookkeepers(genesis.GenesisBookkeepers)
	if err != nil {
		return utils.BYTE_FALSE, err
	}

	//check witness
	err = utils.ValidateOwner(native, operatorAddress)
	if err != nil {
		return utils.BYTE_FALSE, fmt.Errorf("updateConfig, checkWitness error: %v", err)
	}

	configuration := new(Configuration)
	if err := configuration.Deserialization(common.NewZeroCopySource(native.GetInput())); err != nil {
		return utils.BYTE_FALSE, fmt.Errorf("updateConfig, deserialize configuration error: %v", err)
	}

	//get current view
	view, err := GetView(native)
	if err != nil {
		return utils.BYTE_FALSE, fmt.Errorf("getView, get view error: %v", err)
	}
	//get peerPoolMap
	peerPoolMap, err := GetPeerPoolMap(native, view)
	if err != nil {
		return utils.BYTE_FALSE, fmt.Errorf("getPeerPoolMap, get peerPoolMap error: %v", err)
	}
	candidateNum := 0
	for _, peerPoolItem := range peerPoolMap.PeerPoolMap {
		if peerPoolItem.Status == CandidateStatus || peerPoolItem.Status == ConsensusStatus {
			candidateNum = candidateNum + 1
		}
	}

	//check the configuration
	if configuration.C == 0 {
		return utils.BYTE_FALSE, fmt.Errorf("updateConfig. C can not be 0 in config")
	}
	if int(configuration.K) > candidateNum {
		return utils.BYTE_FALSE, fmt.Errorf("updateConfig. K can not be larger than num of candidate peer in config")
	}
	if configuration.L < 16*configuration.K || configuration.L%configuration.K != 0 {
		return utils.BYTE_FALSE, fmt.Errorf("updateConfig. L can not be less than 16*K and K must be times of L in config")
	}
	if configuration.K < 2*configuration.C+1 {
		return utils.BYTE_FALSE, fmt.Errorf("updateConfig. K can not be less than 2*C+1 in config")
	}
	if configuration.N < configuration.K || configuration.K < 7 {
		return utils.BYTE_FALSE, fmt.Errorf("updateConfig. config not match N >= K >= 7")
	}
	if configuration.BlockMsgDelay < 5000 {
		return utils.BYTE_FALSE, fmt.Errorf("updateConfig. BlockMsgDelay must >= 5000")
	}
	if configuration.HashMsgDelay < 5000 {
		return utils.BYTE_FALSE, fmt.Errorf("updateConfig. HashMsgDelay must >= 5000")
	}
	if configuration.PeerHandshakeTimeout < 10 {
		return utils.BYTE_FALSE, fmt.Errorf("updateConfig. PeerHandshakeTimeout must >= 10")
	}
	if configuration.MaxBlockChangeView < 10000 {
		return utils.BYTE_FALSE, fmt.Errorf("updateConfig. MaxBlockChangeView must >= 10000")
	}

	preConfig := &PreConfig{
		Configuration: configuration,
		SetView:       view,
	}
	err = putPreConfig(native, preConfig)
	if err != nil {
		return utils.BYTE_FALSE, fmt.Errorf("putPreConfig, put preConfig error: %v", err)
	}

	return utils.BYTE_TRUE, nil
}

//Update global params of this governance contract
func UpdateGlobalParam(native *native.NativeService) ([]byte, error) {
	// get operator from database
	operatorAddress, err := types.AddressFromBookkeepers(genesis.GenesisBookkeepers)
	if err != nil {
		return utils.BYTE_FALSE, err
	}

	//check witness
	err = utils.ValidateOwner(native, operatorAddress)
	if err != nil {
		return utils.BYTE_FALSE, fmt.Errorf("updateGlobalParam, checkWitness error: %v", err)
	}

	// get config
	config, err := GetConfig(native)
	if err != nil {
		return utils.BYTE_FALSE, fmt.Errorf("getConfig, get config error: %v", err)
	}

	globalParam := new(GlobalParam)
	if err := globalParam.Deserialization(common.NewZeroCopySource(native.GetInput())); err != nil {
		return utils.BYTE_FALSE, fmt.Errorf("updateGlobalParam, deserialize globalParam error: %v", err)
	}

	//check the globalParam
	if globalParam.CandidateNum < 4*config.K {
		return utils.BYTE_FALSE, fmt.Errorf("updateGlobalParam, CandidateNum must >= 4*K")
	}
	if globalParam.MinInitStake < 1 {
		return utils.BYTE_FALSE, fmt.Errorf("updateGlobalParam, MinInitStake must >= 1")
	}
	err = putGlobalParam(native, globalParam)
	if err != nil {
		return utils.BYTE_FALSE, fmt.Errorf("updateGlobalParam, put globalParam error: %v", err)
	}

	return utils.BYTE_TRUE, nil
}

func AddPos(native *native.NativeService) ([]byte, error) {
	params := new(RegisterPeerParam)
	if err := params.Deserialization(common.NewZeroCopySource(native.GetInput())); err != nil {
		return utils.BYTE_FALSE, fmt.Errorf("addPos, contract params deserialize error: %v", err)
	}
	address, err := common.AddressParseFromBytes(params.Address)
	if err != nil {
		return utils.BYTE_FALSE, fmt.Errorf("addPos, common.AddressParseFromBytes error: %v", err)
	}

	//check witness
	err = utils.ValidateOwner(native, address)
	if err != nil {
		return utils.BYTE_FALSE, fmt.Errorf("addPos, checkWitness error: %v", err)
	}

	//get current view
	view, err := GetView(native)
	if err != nil {
		return utils.BYTE_FALSE, fmt.Errorf("addPos, get view error: %v", err)
	}
	//get peerPoolMap
	peerPoolMap, err := GetPeerPoolMap(native, view)
	if err != nil {
		return utils.BYTE_FALSE, fmt.Errorf("addPos, get peerPoolMap error: %v", err)
	}

	peerPoolItem, ok := peerPoolMap.PeerPoolMap[params.PeerPubkey]
	if !ok {
		return utils.BYTE_FALSE, fmt.Errorf("addPos, peerPubkey is not in peerPoolMap")
	}
	if peerPoolItem.Status != ConsensusStatus && peerPoolItem.Status != CandidateStatus {
		return utils.BYTE_FALSE, fmt.Errorf("addPos, peerPubkey is not CandidateStatus or ConsensusStatus")
	}
	if !bytes.Equal(params.Address, peerPoolItem.Address) {
		return utils.BYTE_FALSE, fmt.Errorf("addPos, peerPubkey is not registered by this address")
	}

	peerPoolItem.Pos = peerPoolItem.Pos + params.Pos
	peerPoolMap.PeerPoolMap[params.PeerPubkey] = peerPoolItem
	err = putPeerPoolMap(native, peerPoolMap, view)
	if err != nil {
		return utils.BYTE_FALSE, fmt.Errorf("addPos, put peerPoolMap error: %v", err)
	}

	//ont transfer
	err = appCallTransferOnt(native, address, utils.NodeManagerContractAddress, params.Pos)
	if err != nil {
		return utils.BYTE_FALSE, fmt.Errorf("addPos, ont transfer error: %v", err)
	}
	return utils.BYTE_TRUE, nil
}

func ReducePos(native *native.NativeService) ([]byte, error) {
	params := new(RegisterPeerParam)
	if err := params.Deserialization(common.NewZeroCopySource(native.GetInput())); err != nil {
		return utils.BYTE_FALSE, fmt.Errorf("reducePos, contract params deserialize error: %v", err)
	}
	address, err := common.AddressParseFromBytes(params.Address)
	if err != nil {
		return utils.BYTE_FALSE, fmt.Errorf("reducePos, common.AddressParseFromBytes error: %v", err)
	}

	//check witness
	err = utils.ValidateOwner(native, address)
	if err != nil {
		return utils.BYTE_FALSE, fmt.Errorf("reducePos, checkWitness error: %v", err)
	}

	//get current view
	view, err := GetView(native)
	if err != nil {
		return utils.BYTE_FALSE, fmt.Errorf("reducePos, get view error: %v", err)
	}
	//get peerPoolMap
	peerPoolMap, err := GetPeerPoolMap(native, view)
	if err != nil {
		return utils.BYTE_FALSE, fmt.Errorf("reducePos, get peerPoolMap error: %v", err)
	}

	peerPoolItem, ok := peerPoolMap.PeerPoolMap[params.PeerPubkey]
	if !ok {
		return utils.BYTE_FALSE, fmt.Errorf("reducePos, peerPubkey is not in peerPoolMap")
	}
	if peerPoolItem.Status != ConsensusStatus && peerPoolItem.Status != CandidateStatus {
		return utils.BYTE_FALSE, fmt.Errorf("reducePos, peerPubkey is not CandidateStatus or ConsensusStatus")
	}
	if !bytes.Equal(params.Address, peerPoolItem.Address) {
		return utils.BYTE_FALSE, fmt.Errorf("reducePos, peerPubkey is not registered by this address")
	}

	peerPoolItem.Pos = peerPoolItem.Pos - params.Pos
	peerPoolItem.LockPos = peerPoolItem.LockPos + params.Pos
	peerPoolMap.PeerPoolMap[params.PeerPubkey] = peerPoolItem
	err = putPeerPoolMap(native, peerPoolMap, view)
	if err != nil {
		return utils.BYTE_FALSE, fmt.Errorf("reducePos, put peerPoolMap error: %v", err)
	}

	return utils.BYTE_TRUE, nil
}