package rabbitmq

import "log"

// SetupExchangesAndQueues sets up the exchanges and queues for the microservices
func SetupExchangesAndQueues(client *Client) error {
	// Declare exchanges
	exchanges := []struct {
		name string
		kind string
	}{
		{"orders", "topic"},
		{"inventory", "topic"},
	}

	for _, ex := range exchanges {
		if err := client.DeclareExchange(ex.name, ex.kind); err != nil {
			return err
		}
		log.Printf("Declared exchange: %s (%s)", ex.name, ex.kind)
	}

	// Declare queues
	queues := []string{
		"order.created",
		"order.updated",
		"order.cancelled",
		"inventory.reserved",
		"inventory.released",
		"inventory.updated",
	}

	for _, queue := range queues {
		if err := client.DeclareQueue(queue); err != nil {
			return err
		}
		log.Printf("Declared queue: %s", queue)
	}

	// Bind queues to exchanges
	bindings := []struct {
		queue      string
		exchange   string
		routingKey string
	}{
		{"order.created", "orders", "order.created"},
		{"order.updated", "orders", "order.updated"},
		{"order.cancelled", "orders", "order.cancelled"},
		{"inventory.reserved", "inventory", "inventory.reserved"},
		{"inventory.released", "inventory", "inventory.released"},
		{"inventory.updated", "inventory", "inventory.updated"},
	}

	for _, binding := range bindings {
		if err := client.BindQueue(binding.queue, binding.exchange, binding.routingKey); err != nil {
			return err
		}
		log.Printf("Bound queue %s to exchange %s with key %s", binding.queue, binding.exchange, binding.routingKey)
	}

	return nil
}
