package main

import (
	"log"

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
			{Name: "waitmenu1", Src: []string{"idle"}, Dst: "classmenu"},
			{Name: "waitmenu2", Src: []string{"classmenu"}, Dst: "rvspmenu"},
			{Name: "waitvoice", Src: []string{"idle"}, Dst: "waitvoice"},
			{Name: "waitoxford", Src: []string{"idle"}, Dst: "waitoxford"},
			{Name: "waitidiom", Src: []string{"idle"}, Dst: "waitidiom"},
			{Name: "setvoice", Src: []string{"waitvoice"}, Dst: "idle"},
			{Name: "audio", Src: []string{"idle"}, Dst: "waitaudio"},
			{Name: "setterm", Src: []string{"waitoxford", "waitidiom"}, Dst: "idle"},
			{Name: "cancel", Src: []string{"waitoxford", "waitvoice", "waitaudio", "idle"}, Dst: "idle"},
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
