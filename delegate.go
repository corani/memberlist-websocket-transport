package main

import (
	"log"
	"time"

	"github.com/hashicorp/memberlist"
)

type delegate struct {
	logger *log.Logger
	list   *memberlist.Memberlist
	node   *memberlist.Node
}

func (d *delegate) Start(list *memberlist.Memberlist) {
	d.list = list
	d.node = list.LocalNode()

	if d.node.Name != "app1" {
		d.logger.Println("[INFO] delegate: trying to join app1")

		_, err := d.list.Join([]string{"app1"})
		if err != nil {
			d.logger.Fatalf("[ERROR] delegate: failed to join cluster: %v", err)
		}
	} else {
		go func() {
			for {
				time.Sleep(5 * time.Second)

				for _, member := range list.Members() {
					if err := list.SendBestEffort(member, []byte("Hello from app1")); err != nil {
						d.logger.Printf("[WARN] delegate: failed to send message to %s", member)
					}
				}
			}
		}()
	}
}

func (d *delegate) NodeMeta(limit int) []byte {
	return nil
}

func (d *delegate) NotifyMsg(msg []byte) {
	d.logger.Printf("[INFO] delegate: received msg: %q", string(msg))
}

func (d *delegate) GetBroadcasts(overhead, limit int) [][]byte {
	return nil
}

func (d *delegate) LocalState(join bool) []byte {
	return nil
}

func (d *delegate) MergeRemoteState(buf []byte, join bool) {
}

func (d *delegate) NotifyJoin(node *memberlist.Node) {
	d.logger.Printf("[INFO] delegate: node joined: %s @ %v", node, node.Address())
}

func (d *delegate) NotifyLeave(node *memberlist.Node) {
	d.logger.Printf("[INFO] delegate: node left: %s @ %v", node, node.Address())
}

func (d *delegate) NotifyUpdate(node *memberlist.Node) {
	d.logger.Printf("[INFO] delegate: node updated: %s @ %v", node, node.Address())
}
