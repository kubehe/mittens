//Copyright 2019 Expedia, Inc.
//
//Licensed under the Apache License, Version 2.0 (the "License");
//you may not use this file except in compliance with the License.
//You may obtain a copy of the License at
//
//http://www.apache.org/licenses/LICENSE-2.0
//
//Unless required by applicable law or agreed to in writing, software
//distributed under the License is distributed on an "AS IS" BASIS,
//WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
//See the License for the specific language governing permissions and
//limitations under the License.

package grpc

import (
	"bytes"
	"fmt"
	"log"
	"mittens/pkg/response"
	"os"
	"sync"
	"time"

	"github.com/fullstorydev/grpcurl"
	"github.com/jhump/protoreflect/grpcreflect"
	"golang.org/x/net/context"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
	reflectpb "google.golang.org/grpc/reflection/grpc_reflection_v1alpha"
)

type Client struct {
	host             string
	timeoutSeconds   int
	insecure         bool
	grpcConnectOnce  *sync.Once
	connClose        func() error
	conn             *grpc.ClientConn
	descriptorSource grpcurl.DescriptorSource
}

func NewClient(host string, insecure bool, timeoutSeconds int) Client {
	return Client{host: host, timeoutSeconds: timeoutSeconds, grpcConnectOnce: new(sync.Once), insecure: insecure, connClose: func() error { return nil }}
}

func (c *Client) connect(headers []string) error {

	dialTime := 10 * time.Second

	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(c.timeoutSeconds)*time.Second)
	connCtx, _ := context.WithTimeout(context.Background(), dialTime)

	headersMetadata := grpcurl.MetadataFromHeaders(headers)
	contextWithMetadata := metadata.NewOutgoingContext(ctx, headersMetadata)

	dialOptions := []grpc.DialOption{grpc.WithBlock()}
	if c.insecure {
		log.Print("Grpc client insecure")
		dialOptions = append(dialOptions, grpc.WithInsecure())
	}

	log.Printf("Grpc client connecting to %s", c.host)
	conn, err := grpc.DialContext(connCtx, c.host, dialOptions...)
	if err != nil {
		return fmt.Errorf("Grpc dial: %v", err)
	}

	reflectionClient := grpcreflect.NewClient(contextWithMetadata, reflectpb.NewServerReflectionClient(conn))
	descriptorSource := grpcurl.DescriptorSourceFromServer(contextWithMetadata, reflectionClient)

	log.Print("Grpc client connected")
	c.conn = conn
	c.connClose = func() error { cancel(); return conn.Close() }
	c.descriptorSource = descriptorSource
	return nil
}

// note that the message cannot be null. Even if there's no message to be sent you need to set it to an empty string.
func (c *Client) Request(serviceMethod string, message string, headers []string) response.Response {

	var connErr error
	c.grpcConnectOnce.Do(func() {
		connErr = c.connect(headers)
	})

	if connErr != nil {
		log.Printf("Grpc client connect: %v", connErr)
		return response.Response{Duration: time.Duration(0), Err: connErr, Type: "grpc"}
	}

	in := bytes.NewBufferString(message)

	// TODO - create generic parser and formatter for any request, can we use text parser/formatter?
	requestParser, formatter, err := grpcurl.RequestParserAndFormatterFor(grpcurl.Format("json"), c.descriptorSource, false, false, in)
	if err != nil {
		log.Printf("Cannot construct request parser and formatter for json")
		// FIXME FATAL
		return response.Response{Duration: time.Duration(0), Err: err, Type: "grpc"}
	}
	loggingEventHandler := grpcurl.NewDefaultEventHandler(os.Stdout, c.descriptorSource, formatter, false)
	startTime := time.Now()
	err = grpcurl.InvokeRPC(context.Background(), c.descriptorSource, c.conn, serviceMethod, headers, loggingEventHandler, requestParser.Next)
	endTime := time.Now()
	if err != nil {
		return response.Response{Duration: endTime.Sub(startTime), Err: nil, Type: "grpc"}
	}
	return response.Response{Duration: endTime.Sub(startTime), Err: nil, Type: "grpc"}
}

// Calling close on client that has not established connection does not return error
func (c Client) Close() error {
	log.Print("Closing grpc client connection")
	return c.connClose()
}
