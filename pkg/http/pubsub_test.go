package http_test

import (
	"context"
	"fmt"
	nethttp "net/http"
	"testing"
	"time"

	"github.com/ThreeDotsLabs/watermill/message/subscriber"
	"github.com/ThreeDotsLabs/watermill/pubsub/tests"

	"github.com/ThreeDotsLabs/watermill/message"

	"github.com/stretchr/testify/require"

	"github.com/ThreeDotsLabs/watermill"
	"github.com/ThreeDotsLabs/watermill-http/v2/pkg/http"
)

func createPubSub(t *testing.T) (*http.Publisher, *http.Subscriber) {
	logger := watermill.NewStdLogger(true, true)

	// use any free port to allow parallel tests
	sub, err := http.NewSubscriber(":0", http.SubscriberConfig{}, logger)
	require.NoError(t, err)

	publisherConf := http.PublisherConfig{
		MarshalMessageFunc: http.DefaultMarshalMessageFunc,
	}

	pub, err := http.NewPublisher(publisherConf, logger)
	require.NoError(t, err)

	return pub, sub
}

func TestPublishSubscribe(t *testing.T) {
	t.Skip("todo - fix")

	tests.TestPubSub(
		t,
		tests.Features{
			ConsumerGroups:      false,
			ExactlyOnceDelivery: true,
			GuaranteedOrder:     true,
			Persistent:          false,
		},
		nil,
		nil,
	)
}

func TestHttpPubSub(t *testing.T) {
	pub, sub := createPubSub(t)

	defer func() {
		require.NoError(t, pub.Close())
		require.NoError(t, sub.Close())
	}()

	msgs, err := sub.Subscribe(context.Background(), "/test")
	require.NoError(t, err)

	go sub.StartHTTPServer()

	waitForHTTP(t, sub, time.Second*10)

	t.Run("publish a message with invalid metadata", func(t *testing.T) {
		req, err := nethttp.NewRequest(nethttp.MethodPost, fmt.Sprintf("http://%s/test", sub.Addr()), nil)
		require.NoError(t, err)

		req.Header.Set("Content-Type", "application/json")
		req.Header.Set(http.HeaderMetadata, "invalid_metadata")

		resp, err := nethttp.DefaultClient.Do(req)
		require.NoError(t, err)

		require.Equal(t, nethttp.StatusBadRequest, resp.StatusCode)
	})

	t.Run("publish correct simple messages", func(t *testing.T) {
		receivedMessages := make(chan message.Messages)

		go func() {
			received, _ := subscriber.BulkRead(msgs, 100, time.Second*10)
			receivedMessages <- received
		}()

		publishedMessages := tests.PublishSimpleMessages(t, 100, pub, fmt.Sprintf("http://%s/test", sub.Addr()))

		tests.AssertAllMessagesReceived(t, publishedMessages, <-receivedMessages)
	})
}

func waitForHTTP(t *testing.T, sub *http.Subscriber, timeoutTime time.Duration) {
	timeout := time.After(timeoutTime)
	for {
		addr := sub.Addr()
		if addr != nil {
			break
		}

		select {
		case <-timeout:
			t.Fatal("server not up")
		default:
			// ok
		}

		time.Sleep(time.Millisecond * 10)
	}
}
