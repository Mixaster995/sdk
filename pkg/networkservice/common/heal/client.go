// Copyright (c) 2021 Cisco and/or its affiliates.
//
// SPDX-License-Identifier: Apache-2.0
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at:
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package heal

import (
	"context"

	"github.com/networkservicemesh/api/pkg/api/networkservice"
	"github.com/pkg/errors"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/types/known/emptypb"

	"github.com/networkservicemesh/sdk/pkg/networkservice/common/clientconn"
	"github.com/networkservicemesh/sdk/pkg/networkservice/core/next"
	"github.com/networkservicemesh/sdk/pkg/tools/extend"
	"github.com/networkservicemesh/sdk/pkg/tools/postpone"
)

type healClient struct {
	chainCtx        context.Context
	livelinessCheck LivelinessCheck
}

// NewClient - returns a new heal client chain element
func NewClient(chainCtx context.Context, opts ...Option) networkservice.NetworkServiceClient {
	o := &option{
		livelinessCheck: func(_ *networkservice.Connection) bool { return false },
	}
	for _, opt := range opts {
		opt(o)
	}
	return &healClient{
		chainCtx:        chainCtx,
		livelinessCheck: o.livelinessCheck,
	}
}

func (h *healClient) Request(ctx context.Context, request *networkservice.NetworkServiceRequest, opts ...grpc.CallOption) (*networkservice.Connection, error) {
	closeCtxFunc := postpone.ContextWithValues(ctx)
	// Cancel any existing eventLoop
	if cancelEventLoop, loaded := loadAndDelete(ctx); loaded {
		cancelEventLoop()
	}

	conn, err := next.Client(ctx).Request(ctx, request, opts...)
	if err != nil {
		return nil, err
	}
	cc, ccLoaded := clientconn.Load(ctx)
	if ccLoaded {
		cancelEventLoop, eventLoopErr := newEventLoop(extend.WithValuesFromContext(h.chainCtx, ctx), cc, conn, h.livelinessCheck)
		if eventLoopErr != nil {
			closeCtx, closeCancel := closeCtxFunc()
			defer closeCancel()
			_, _ = next.Client(closeCtx).Close(closeCtx, conn)
			return nil, errors.Wrap(eventLoopErr, "unable to monitor")
		}
		store(ctx, cancelEventLoop)
	}
	return conn, nil
}

func (h *healClient) Close(ctx context.Context, conn *networkservice.Connection, opts ...grpc.CallOption) (*emptypb.Empty, error) {
	// Cancel any existing eventLoop
	if cancelEventLoop, loaded := loadAndDelete(ctx); loaded {
		cancelEventLoop()
	}
	return next.Client(ctx).Close(ctx, conn)
}
