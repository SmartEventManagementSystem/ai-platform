package services

import (
	"context"
	"encoding/json"
	"time"

	"github.com/IBM/sarama"
	"go.uber.org/zap"
)

type KafkaProducer struct {
	producer sarama.SyncProducer
	brokers  []string
	logger   *zap.Logger
}

func NewKafkaProducer(brokers string, logger *zap.Logger) (*KafkaProducer, error) {
	config := sarama.NewConfig()
	config.Producer.RequiredAcks = sarama.WaitForAll
	config.Producer.Retry.Max = 5
	config.Producer.Return.Successes = true
	config.Producer.Compression = sarama.CompressionSnappy
	config.Net.DialTimeout = 10 * time.Second
	config.Net.ReadTimeout = 10 * time.Second
	config.Net.WriteTimeout = 10 * time.Second

	brokerList := []string{brokers}
	producer, err := sarama.NewSyncProducer(brokerList, config)
	if err != nil {
		return nil, err
	}

	return &KafkaProducer{
		producer: producer,
		brokers:  brokerList,
		logger:   logger,
	}, nil
}

func (p *KafkaProducer) Publish(topic string, event interface{}) error {
	data, err := json.Marshal(event)
	if err != nil {
		return err
	}

	msg := &sarama.ProducerMessage{
		Topic: topic,
		Value: sarama.ByteEncoder(data),
	}

	partition, offset, err := p.producer.SendMessage(msg)
	if err != nil {
		p.logger.Error("Failed to publish message",
			zap.String("topic", topic),
			zap.Error(err))
		return err
	}

	p.logger.Debug("Message published",
		zap.String("topic", topic),
		zap.Int32("partition", partition),
		zap.Int64("offset", offset))

	return nil
}

func (p *KafkaProducer) PublishWithKey(topic, key string, event interface{}) error {
	data, err := json.Marshal(event)
	if err != nil {
		return err
	}

	msg := &sarama.ProducerMessage{
		Topic: topic,
		Key:   sarama.StringEncoder(key),
		Value: sarama.ByteEncoder(data),
	}

	partition, offset, err := p.producer.SendMessage(msg)
	if err != nil {
		return err
	}

	p.logger.Debug("Message published with key",
		zap.String("topic", topic),
		zap.String("key", key),
		zap.Int32("partition", partition),
		zap.Int64("offset", offset))

	return nil
}

func (p *KafkaProducer) Close() error {
	if p.producer != nil {
		return p.producer.Close()
	}
	return nil
}

type KafkaConsumer struct {
	consumer sarama.Consumer
	logger   *zap.Logger
}

func NewKafkaConsumer(brokers string, logger *zap.Logger) (*KafkaConsumer, error) {
	config := sarama.NewConfig()
	config.Consumer.Return.Errors = true

	consumer, err := sarama.NewConsumer([]string{brokers}, config)
	if err != nil {
		return nil, err
	}

	return &KafkaConsumer{
		consumer: consumer,
		logger:   logger,
	}, nil
}

func (c *KafkaConsumer) Consume(topic string, handler func([]byte) error) error {
	partitions, err := c.consumer.Partitions(topic)
	if err != nil {
		return err
	}

	for _, partition := range partitions {
		pc, err := c.consumer.ConsumePartition(topic, partition, sarama.OffsetNewest)
		if err != nil {
			return err
		}

		go func(pc sarama.PartitionConsumer) {
			for msg := range pc.Messages() {
				if err := handler(msg.Value); err != nil {
					c.logger.Error("Message handler error",
						zap.Error(err),
						zap.Int64("offset", msg.Offset))
				}
			}
		}(pc)
	}

	return nil
}

func (c *KafkaConsumer) Close() error {
	if c.consumer != nil {
		return c.consumer.Close()
	}
	return nil
}

// PublishChatEvent publishes chat events to Kafka
func (p *KafkaProducer) PublishChatEvent(ctx context.Context, eventType string, data map[string]interface{}) error {
	event := map[string]interface{}{
		"type":      eventType,
		"timestamp": time.Now().UTC().Format(time.RFC3339),
		"data":      data,
	}
	return p.Publish("chat.events", event)
}

// PublishRAGEvent publishes RAG events to Kafka
func (p *KafkaProducer) PublishRAGEvent(ctx context.Context, eventType string, data map[string]interface{}) error {
	event := map[string]interface{}{
		"type":      eventType,
		"timestamp": time.Now().UTC().Format(time.RFC3339),
		"data":      data,
	}
	return p.Publish("rag.events", event)
}

// PublishMCPToolEvent publishes MCP tool events to Kafka
func (p *KafkaProducer) PublishMCPToolEvent(ctx context.Context, toolName string, success bool, latency time.Duration, data map[string]interface{}) error {
	event := map[string]interface{}{
		"type":      "mcp_tool_call",
		"tool":      toolName,
		"success":   success,
		"latency":   latency.String(),
		"timestamp": time.Now().UTC().Format(time.RFC3339),
		"data":      data,
	}
	return p.Publish("mcp.events", event)
}
