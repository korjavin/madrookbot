package main

import (
	"github.com/looplab/fsm"
	"log"
)

type dialogue struct {
	FromID int
	state  *fsm.FSM
}

func newDialogue(fid int) *dialogue {
	d := dialogue{FromID: fid}
	d.state = fsm.NewFSM(
		"idle",
		fsm.Events{
			{Name: "waitvoice", Src: []string{"idle"}, Dst: "waitvoice"},
			{Name: "waitoxford", Src: []string{"idle"}, Dst: "waitoxford"},
			{Name: "waitidiom", Src: []string{"idle"}, Dst: "waitidiom"},
			{Name: "waitterm", Src: []string{"idle"}, Dst: "waitterm"},
			{Name: "setvoice", Src: []string{"waitvoice"}, Dst: "idle"},
			{Name: "setterm", Src: []string{"waitterm", "waitoxford", "waitidiom"}, Dst: "idle"},
			{Name: "cancel", Src: []string{"waitoxford", "waitterm", "waitvoice", "idle"}, Dst: "idle"},
		},
		fsm.Callbacks{
			"enter_state": func(e *fsm.Event) { d.enterState(e) },
		},
	)
	return &d
}
func (d *dialogue) enterState(e *fsm.Event) {
	log.Printf("The dialogue to %d is %s\n", d.FromID, e.Dst)
}

func (d *dialogue) Event(ev string) error {
	return nil
}
