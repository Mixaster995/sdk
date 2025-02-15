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

package retry

import (
	"context"
	"time"

	"github.com/networkservicemesh/api/pkg/api/registry"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/types/known/emptypb"

	"github.com/networkservicemesh/sdk/pkg/registry/core/next"
	"github.com/networkservicemesh/sdk/pkg/tools/clock"
	"github.com/networkservicemesh/sdk/pkg/tools/log"
)

type retryNSClient struct {
	interval   time.Duration
	tryTimeout time.Duration
}

// NewNetworkServiceRegistryClient - returns a retry chain element
func NewNetworkServiceRegistryClient(opts ...Option) registry.NetworkServiceRegistryClient {
	clientOpts := &options{
		interval:   time.Millisecond * 200,
		tryTimeout: time.Second * 15,
	}

	for _, opt := range opts {
		opt(clientOpts)
	}

	return &retryNSClient{
		interval:   clientOpts.interval,
		tryTimeout: clientOpts.tryTimeout,
	}
}

func (r *retryNSClient) Register(ctx context.Context, in *registry.NetworkService, opts ...grpc.CallOption) (*registry.NetworkService, error) {
	logger := log.FromContext(ctx).WithField("retryNSClient", "Register")
	c := clock.FromContext(ctx)

	for ctx.Err() == nil {
		registerCtx, cancel := c.WithTimeout(ctx, r.tryTimeout)
		resp, err := next.NetworkServiceRegistryClient(registerCtx).Register(registerCtx, in, opts...)
		cancel()

		if err != nil {
			logger.Errorf("try attempt has failed: %v", err.Error())

			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-c.After(r.interval):
				continue
			}
		}

		return resp, err
	}

	return nil, ctx.Err()
}

func (r *retryNSClient) Find(ctx context.Context, query *registry.NetworkServiceQuery, opts ...grpc.CallOption) (registry.NetworkServiceRegistry_FindClient, error) {
	logger := log.FromContext(ctx).WithField("retryNSClient", "Find")
	c := clock.FromContext(ctx)

	for ctx.Err() == nil {
		stream, err := next.NetworkServiceRegistryClient(ctx).Find(ctx, query, opts...)

		if err != nil {
			logger.Errorf("try attempt has failed: %v", err.Error())
			<-c.After(r.interval)
			continue
		}

		return stream, err
	}

	return nil, ctx.Err()
}

func (r *retryNSClient) Unregister(ctx context.Context, in *registry.NetworkService, opts ...grpc.CallOption) (*emptypb.Empty, error) {
	logger := log.FromContext(ctx).WithField("retryNSClient", "Unregister")
	c := clock.FromContext(ctx)

	for ctx.Err() == nil {
		closeCtx, cancel := c.WithTimeout(ctx, r.tryTimeout)
		resp, err := next.NetworkServiceRegistryClient(closeCtx).Unregister(closeCtx, in, opts...)
		cancel()

		if err != nil {
			logger.Errorf("try attempt has failed: %v", err.Error())

			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-c.After(r.interval):
				continue
			}
		}

		return resp, err
	}

	return nil, ctx.Err()
}
