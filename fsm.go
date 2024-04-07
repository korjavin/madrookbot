package main

import (
	"github.com/looplab/fsm"
)

type dialogue struct {
	FromID int
	state  *fsm.FSM
	MsgID  int
}

func newDialogue(fid int) *dialogue {
	d := dialogue{FromID: fid}
	d.state = fsm.NewFSM(
		"idle",
		fsm.Events{
			{Name: "waitvoice", Src: []string{"idle"}, Dst: "waitvoice"},
			{Name: "waitidiom", Src: []string{"idle"}, Dst: "waitidiom"},
			{Name: "cancel", Src: []string{"waitoxford", "waitvoice", "idle"}, Dst: "idle"},
		},
		fsm.Callbacks{},
	)
	return &d
}

func (d *dialogue) Event(ev string) error {
	return nil
}
