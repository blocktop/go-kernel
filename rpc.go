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
	"errors"
	"math/rand"
	"net/http"
	"time"

	rpcclient "github.com/blocktop/go-rpc-client/kernel"
)

func init() {
	rand.Seed(time.Now().UnixNano())
}

type RPC struct {
}

func (h *RPC) GetMetrics(r *http.Request, args *rpcclient.GetMetricsArgs, reply *rpcclient.GetMetricsReply) error {
	if !Initialized() {
		return errors.New("kernel not initialized")
	}
	switch args.Format {
	case "text":
		reply.Metrics = metrics.String()

	case "json":
		j, err := metrics.JSON()
		if err != nil {
			return err
		}
		reply.Metrics = j

	default:
		return errors.New("format must be either text or json")
	}

	return nil
}
