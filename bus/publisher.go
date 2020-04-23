package bus

import (
	"context"
	"encoding/json"
	"fmt"

	servicebus "github.com/Azure/azure-service-bus-go"
	"github.com/juan-lee/carp/messages/workers"
)

type ServiceBusPublisher struct {
	topicSender *servicebus.Sender
}

type PublisherConfig struct {
	Region                     string
	Environment                string
	UnderlayName               string
	ServiceBusConnectionString string
}

func NewPublisher(ctx context.Context, cfg *PublisherConfig) (Publisher, error) {
	// Setup necessary SB resources
	namespace, err := getNamespace(cfg.ServiceBusConnectionString)
	if err != nil {
		return nil, fmt.Errorf("failed to get namespace from provided ServiceBus connection string: %w", err)
	}
	topicEntity, err := getTopicEntity(ctx, cfg.Environment, cfg.Region, namespace)
	if err != nil {
		return nil, fmt.Errorf("failed to get topicEntity: %w", err)
	}

	// Generate new topic client
	topic, err := namespace.NewTopic(topicEntity.Name)
	if err != nil {
		return nil, fmt.Errorf("failed to create new topic %s: %w", topicEntity.Name, err)
	}
	defer func() {
		_ = topic.Close(ctx)
	}()

	topicSender, err := topic.NewSender(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to create new topic sender for topic %s: %w", topicEntity.Name, err)
	}
	return &ServiceBusPublisher{topicSender}, nil
}

// Publish sends a message to a topic based on region and environment
func (p *ServiceBusPublisher) Publish(ctx context.Context, command workers.PutCluster) error {
	commandStr, err := json.Marshal(command)

	// Adding in user properties to enable filtering on receiver side
	msg := servicebus.NewMessageFromString(string(commandStr))
	msg.UserProperties = make(map[string]interface{})
	msg.UserProperties["destinationId"] = command.DestinationId
	err = p.topicSender.Send(ctx, msg)
	if err != nil {
		return fmt.Errorf("failed to send message to topic %s: %w", p.topicSender.Name, err)
	}
	return nil
}
