package consensus

import (
	"encoding/json"
	"errors"
	"fmt"
	"time"
)

type State struct {
	ViewID         int64
	MsgLogs        *MsgLogs
	LastSequenceID int64
	CurrentStage   Stage
}

type MsgLogs struct {
	ReqMsg      *RequestMsg
	PrepareMsgs map[string]*VoteMsg
	CommitMsgs  map[string]*VoteMsg
}

type Stage int

const (
	Idle        Stage = iota // 节点被成功创建，但共识还没有开始
	PrePrepared              // Request 被成功处理，但是还没有到达prepare阶段
	Prepared                 // 原论文中的 prepared 状态
	Committed                // 原论文中的 committed-local 状态
)

// f: 拜占庭错误节点的数量
// f = (n - ­1) / 3
// 本例中n = 7.
const f = 2

// 如果不存在 lastSequenceID，则赋值为-1
func CreateState(viewID int64, lastSequenceID int64) *State {
	return &State{
		ViewID: viewID,
		MsgLogs: &MsgLogs{
			ReqMsg:      nil,
			PrepareMsgs: make(map[string]*VoteMsg),
			CommitMsgs:  make(map[string]*VoteMsg),
		},
		LastSequenceID: lastSequenceID,
		CurrentStage:   Idle,
	}
}

func (state *State) StartConsensus(request *RequestMsg) (*PrePrepareMsg, error) {
	// 用时间戳设置唯一的 sequenceID
	sequenceID := time.Now().UnixNano()
	if state.LastSequenceID != -1 {
		for state.LastSequenceID >= sequenceID {
			sequenceID += 1
		}
	}
	request.SequenceID = sequenceID

	// 将request保存至 MsgLogs
	state.MsgLogs.ReqMsg = request

	// 杂凑 request 内容
	digest, err := digest(request)
	if err != nil {
		fmt.Println(err)
		return nil, err
	}

	// 将状态更改为 prepared
	state.CurrentStage = PrePrepared

	// 新建并返回 PrePrepareMsg，用于 PBFT 下一阶段
	return &PrePrepareMsg{
		ViewID:     state.ViewID,
		SequenceID: sequenceID,
		Digest:     digest,
		RequestMsg: request,
	}, nil
}

func (state *State) PrePrepare(prePrepareMsg *PrePrepareMsg) (*VoteMsg, error) {
	// 将 prePrepareMsg 中的 RequestMsg 保存到 MsgLogs 中
	state.MsgLogs.ReqMsg = prePrepareMsg.RequestMsg

	// 检查 prePrepareMsg 各项字段是否正确
	if !state.verifyMsg(prePrepareMsg.ViewID, prePrepareMsg.SequenceID, prePrepareMsg.Digest) {
		return nil, errors.New("pre-prepare message is corrupted")
	}

	// 将状态更改为 PrePrepared
	state.CurrentStage = PrePrepared

	// 新建并返回 prepareMsg，用于 PBFT 下一阶段
	// 如果某个从结点验证通过了某条 PrePrepared，那么它将进入 prepare 阶段
	// 故此处无需检查是否收到超过 2f 个节点的 prePrepareMsg
	return &VoteMsg{
		ViewID:     state.ViewID,
		SequenceID: prePrepareMsg.SequenceID,
		Digest:     prePrepareMsg.Digest,
		MsgType:    PrepareMsg,
	}, nil
}

func (state *State) Prepare(prepareMsg *VoteMsg) (*VoteMsg, error) {
	if !state.verifyMsg(prepareMsg.ViewID, prepareMsg.SequenceID, prepareMsg.Digest) {
		return nil, errors.New("prepare message is corrupted")
	}

	// 将 PrepareMsgs 保存到 log 中
	state.MsgLogs.PrepareMsgs[prepareMsg.NodeID] = prepareMsg

	// 打印当前投票情况
	fmt.Printf("[Prepare-Vote]: %d\n", len(state.MsgLogs.PrepareMsgs))

	// 如果收到超过 2f 个节点的 PrepareMsgs
	if state.prepared() {
		// 将状态更改为 Prepared
		state.CurrentStage = Prepared

		// 新建并返回 CommitMsg，用于 PBFT 下一阶段
		return &VoteMsg{
			ViewID:     state.ViewID,
			SequenceID: prepareMsg.SequenceID,
			Digest:     prepareMsg.Digest,
			MsgType:    CommitMsg,
		}, nil
	}

	return nil, nil
}

func (state *State) Commit(commitMsg *VoteMsg) (*ReplyMsg, *RequestMsg, error) {
	if !state.verifyMsg(commitMsg.ViewID, commitMsg.SequenceID, commitMsg.Digest) {
		return nil, nil, errors.New("commit message is corrupted")
	}

	// 将 commitMsg 保存到 log 中
	state.MsgLogs.CommitMsgs[commitMsg.NodeID] = commitMsg

	// 打印当前投票情况
	fmt.Printf("[Commit-Vote]: %d\n", len(state.MsgLogs.CommitMsgs))

	// 如果收到超过 2f 个节点的 commitMsg
	if state.committed() {
		// 本地执行请求并得到结果
		result := "Executed"

		// 将状态更改为 Committed
		state.CurrentStage = Committed

		// 返回 ReplyMsg 和 ReqMsg
		return &ReplyMsg{
			ViewID:    state.ViewID,
			Timestamp: state.MsgLogs.ReqMsg.Timestamp,
			ClientID:  state.MsgLogs.ReqMsg.ClientID,
			Result:    result,
		}, state.MsgLogs.ReqMsg, nil
	}

	return nil, nil, nil
}

//---------工具函数---------

func (state *State) verifyMsg(viewID int64, sequenceID int64, digestGot string) bool {
	// Wrong view. That is, wrong configurations of peers to start the consensus.
	if state.ViewID != viewID {
		return false
	}

	// Check if the Primary sent fault sequence number. => Faulty primary.
	// TODO: adopt upper/lower bound check.
	if state.LastSequenceID != -1 {
		if state.LastSequenceID >= sequenceID {
			return false
		}
	}

	digest, err := digest(state.MsgLogs.ReqMsg)
	if err != nil {
		fmt.Println(err)
		return false
	}

	// Check digest.
	if digestGot != digest {
		return false
	}

	return true
}

func (state *State) prepared() bool {
	if state.MsgLogs.ReqMsg == nil {
		return false
	}

	if len(state.MsgLogs.PrepareMsgs) < 2*f {
		return false
	}

	return true
}

func (state *State) committed() bool {
	if !state.prepared() {
		return false
	}

	if len(state.MsgLogs.CommitMsgs) < 2*f {
		return false
	}

	return true
}

func digest(object interface{}) (string, error) {
	msg, err := json.Marshal(object)

	if err != nil {
		return "", err
	}

	return Hash(msg), nil
}
