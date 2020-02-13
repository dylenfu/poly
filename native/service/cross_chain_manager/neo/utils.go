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

package neo

//func verifyFromNEOTx(native *native.NativeService, proof, txHash []byte, fromChainid uint64, height uint32) (*scom.MakeTxParam, error) {
////get block header
//header, err := neo.GetHeaderByHeight(native, fromChainid, height)
//if err != nil {
//	return nil, fmt.Errorf("verifyFromOntTx, get header by height %d from chain %d error: %v",
//		height, fromChainid, err)
//}
//
//v, err := merkle.MerkleProve(proof, header.CrossStatesRoot.Bytes())
//if err != nil {
//	return nil, fmt.Errorf("VerifyFromOntTx, merkle.MerkleProve verify merkle proof error")
//}
//
//s := common.NewZeroCopySource(v)
//txParam := new(scom.MakeTxParam)
//if err := txParam.Deserialization(s); err != nil {
//	return nil, fmt.Errorf("VerifyFromOntTx, deserialize merkleValue error:%s", err)
//}
//if !bytes.Equal(txHash, txParam.TxHash) {
//	return nil, fmt.Errorf("VerifyFromOntTx, relayer txHash:%x doesn't equal user txHash:%x", txParam.TxHash, txParam.TxHash)
//}
//return txParam, nil
//}