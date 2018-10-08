// Copyright © 2018 J. Strobus White.
// This file is part of the blocktop blockchain development kit.
//
// Blocktop is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// Blocktop is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU General Public License for more details.
//
// You should have received a copy of the GNU General Public License
// along with blocktop. If not, see <http://www.gnu.org/licenses/>.

package kernel

import (
	"reflect"

	spec "github.com/blocktop/go-spec"
)

type MessageChannel struct {
	Prototype      spec.Marshalled
	Protocol       *spec.MessageProtocol
	ReceiveHandler spec.MessageReceiver
}

func NewMessageChannel(prototype spec.Marshalled, receiveHandler spec.MessageReceiver) *MessageChannel {
	c := &MessageChannel{}
	c.Prototype = prototype
	c.Protocol = spec.NewProtocolMarshalled(kernel.name, prototype)
	c.ReceiveHandler = receiveHandler
	return c
}

func (c *MessageChannel) marshal(item spec.Marshalled) (*spec.NetworkMessage, error) {
	data, links, err := item.Marshal()
	if err != nil {
		return nil, err
	}

	netMsg := &spec.NetworkMessage{
		Data:     data,
		Links:    links,
		Hash:     item.Hash(),
		Protocol: c.Protocol,
		From:     kernel.net.PeerID()}

	return netMsg, nil
}

func (c *MessageChannel) unmarshal(netMsg *spec.NetworkMessage) (spec.Marshalled, error) {
	var item spec.Marshalled = reflect.New(reflect.ValueOf(c.Prototype).Elem().Type()).Interface().(spec.Marshalled)
	err := item.Unmarshal(netMsg.Data, netMsg.Links)
	if err != nil {
		return nil, err
	}
	return item, nil
}
