// Package kafka provides Kafka client for message publishing and consuming.
//
//nolint:revive
package kafka

//go:generate go run go.uber.org/mock/mockgen -source=./kafka.go -destination=./mocks/kafka_mock.go -package=mocks

import (
	"context"
	"encoding/json"
	"fmt"
	"go-espn-api/config"
	"net"

	"github.com/rs/zerolog/log"
	kafkaGo "github.com/segmentio/kafka-go"
	"github.com/segmentio/kafka-go/sasl/plain"
)

// Message represents a Kafka message with key and value.
type Message struct {
	Key   string
	Value any
}

// ToKafkaMessage converts the Message to a kafka-go Message.
func (m *Message) ToKafkaMessage() (kafkaGo.Message, error) {
	value := m.Value

	jsonValue, err := json.Marshal(value)
	if err != nil {
		log.Error().Err(err).Msg("Failed to marshal message value to JSON")

		return kafkaGo.Message{}, fmt.Errorf("failed to marshal message value to JSON: %w", err)
	}

	message := kafkaGo.Message{
		Key:   []byte(m.Key),
		Value: jsonValue,
	}

	return message, nil
}

// DecodeKafkaMessage decodes a Kafka message into a generic type.
func DecodeKafkaMessage[T any](msg kafkaGo.Message) (Message, error) {
	var zero T

	err := json.Unmarshal(msg.Value, &zero)
	if err != nil {
		log.Error().Err(err).Msg("Failed to unmarshal Kafka message value from JSON")

		return Message{}, fmt.Errorf("failed to unmarshal Kafka message value from JSON: %w", err)
	}

	return Message{
		Key:   string(msg.Key),
		Value: zero,
	}, nil
}

// Client defines the interface for Kafka operations.
type Client interface {
	SendMessages(ctx context.Context, topic string, messages ...Message) (err error)
	Consume(ctx context.Context, consumerGroup, topic string, handler func(message kafkaGo.Message))
	Reader(consumerGroup, topic string) *kafkaGo.Reader
}

type kafkaClientImpl struct {
	config    *config.Config
	dialer    *kafkaGo.Dialer
	transport *kafkaGo.Transport
	address   net.Addr
}

func New(config *config.Config) Client {
	mechanism := plain.Mechanism{
		Username: config.Kafka.SASL.Username,
		Password: config.Kafka.SASL.Password,
	}

	dialer := &kafkaGo.Dialer{
		DualStack:     true,
		SASLMechanism: mechanism,
	}

	transport := &kafkaGo.Transport{
		SASL: mechanism,
	}

	log.Info().Msg("Kafka client initialzed")

	return &kafkaClientImpl{
		config:    config,
		dialer:    dialer,
		transport: transport,
		address:   kafkaGo.TCP(config.Kafka.Brokers...),
	}
}

func (k *kafkaClientImpl) Reader(consumerGroup, topic string) *kafkaGo.Reader {
	if topic == "" {
		log.Error().Msg("Topic name cannot be empty when creating Kafka reader")

		return nil
	}

	groupID := k.config.Kafka.ConsumerGroup
	if consumerGroup != "" {
		groupID = consumerGroup
	}

	return kafkaGo.NewReader(kafkaGo.ReaderConfig{
		Brokers:     k.config.Kafka.Brokers,
		Topic:       topic,
		GroupID:     groupID,
		Dialer:      k.dialer,
		StartOffset: kafkaGo.FirstOffset,
	})
}

func (k *kafkaClientImpl) SendMessages(ctx context.Context, topic string, messages ...Message) (err error) {
	msgs := []kafkaGo.Message{}

	writer := &kafkaGo.Writer{
		Addr:                   k.address,
		Topic:                  topic,
		Transport:              k.transport,
		AllowAutoTopicCreation: true,
		Async:                  true,
	}

	for _, message := range messages {
		msg, err := message.ToKafkaMessage()
		if err != nil {
			log.Error().Err(err).Str("topic", topic).Msg("Failed to convert message to Kafka message.")

			return fmt.Errorf("failed to convert message to Kafka message: %w", err)
		}

		msgs = append(msgs, msg)
	}

	err = writer.WriteMessages(ctx, msgs...)
	if err != nil {
		log.Error().Err(err).Str("topic", topic).Msg("Failed to send message to Kafka.")

		return fmt.Errorf("failed to send message to Kafka: %w", err)
	}

	log.Info().Str("topic", topic).Msg("Sent message successfully.")

	return nil
}

func (k *kafkaClientImpl) Consume(ctx context.Context, consumerGroup, topic string, handler func(message kafkaGo.Message)) {
	reader := k.Reader(consumerGroup, topic)
	if reader == nil {
		log.Error().Msg("Failed to create Kafka reader")

		return
	}

	for {
		select {
		case <-ctx.Done():
			log.Info().Msg("Consumer context done.")

			err := reader.Close()
			if err != nil {
				log.Error().Err(err).Msg("Failed to close Kafka reader.")
			}

			return
		default:
			msg, err := reader.ReadMessage(ctx)
			if err != nil {
				log.Error().Err(err).Str("topic", topic).Msg("Failed to read message from Kafka.")

				continue
			}

			log.Info().Str("topic", topic).Str("key", string(msg.Key)).Msg("Received message from Kafka.")

			go handler(msg)
		}
	}
}
