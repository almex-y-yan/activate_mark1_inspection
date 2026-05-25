package core

import (
	"errors"
	"fmt"
)

const (
	TargetCard = "card"
	TargetIRS  = "irs"
	TargetNM43 = "nm43"
)

type BasicTargetInput struct {
	Selected bool `json:"selected"`
	Com      *int `json:"com"`
}

type IrsTargetInput struct {
	Selected   bool `json:"selected"`
	Device1Com *int `json:"device1Com"`
	UseDevice2 bool `json:"useDevice2"`
	Device2Com *int `json:"device2Com"`
}

type ApplyRequest struct {
	Card BasicTargetInput `json:"card"`
	IRS  IrsTargetInput   `json:"irs"`
	NM43 BasicTargetInput `json:"nm43"`
}

type Operation struct {
	Target     string
	Device1Com int
	UseDevice2 bool
	Device2Com *int
}

func BuildOperations(req ApplyRequest) ([]Operation, error) {
	ops := make([]Operation, 0, 3)

	cardOps, err := buildBasicTargetOp(TargetCard, req.Card)
	if err != nil {
		return nil, err
	}
	ops = append(ops, cardOps...)

	irsOps, err := buildIrsOp(req.IRS)
	if err != nil {
		return nil, err
	}
	ops = append(ops, irsOps...)

	nm43Ops, err := buildBasicTargetOp(TargetNM43, req.NM43)
	if err != nil {
		return nil, err
	}
	ops = append(ops, nm43Ops...)

	if len(ops) == 0 {
		return nil, errors.New("対象が未選択です。1つ以上選択してください。")
	}
	return ops, nil
}

func buildBasicTargetOp(target string, in BasicTargetInput) ([]Operation, error) {
	if !in.Selected {
		return []Operation{}, nil
	}
	if in.Com == nil {
		return nil, fmt.Errorf("%s のComが未指定です", target)
	}
	if *in.Com <= 0 {
		return nil, fmt.Errorf("%s のComは1以上で指定してください", target)
	}
	return []Operation{{
		Target:     target,
		Device1Com: *in.Com,
	}}, nil
}

func buildIrsOp(in IrsTargetInput) ([]Operation, error) {
	if !in.Selected {
		return []Operation{}, nil
	}
	if in.Device1Com == nil {
		return nil, errors.New("irs のDEVICE1 Comが未指定です")
	}
	if *in.Device1Com <= 0 {
		return nil, errors.New("irs のDEVICE1 Comは1以上で指定してください")
	}
	if in.UseDevice2 && in.Device2Com == nil {
		return nil, errors.New("irs のDEVICE2使用時はComが必要です")
	}
	if in.Device2Com != nil && *in.Device2Com <= 0 {
		return nil, errors.New("irs のDEVICE2 Comは1以上で指定してください")
	}
	return []Operation{{
		Target:     TargetIRS,
		Device1Com: *in.Device1Com,
		UseDevice2: in.UseDevice2,
		Device2Com: in.Device2Com,
	}}, nil
}
