package rabbitmq

import "log"

func SetupExchangesAndQueues(client *Client) error {
	exchanges := []struct {
		name string
		kind string
	}{
		{"orders", "topic"},
		{"inventory", "topic"},
		{"warehouse", "topic"},
	}

	for _, ex := range exchanges {
		if err := client.DeclareExchange(ex.name, ex.kind); err != nil {
			return err
		}
		log.Printf("Declared exchange: %s (%s)", ex.name, ex.kind)
	}

	queues := []string{
		"order.created",
		"order.updated",
		"order.cancelled",
		"inventory.reserved",
		"inventory.released",
		"inventory.updated",
		"warehouse.test",
	}

	for _, queue := range queues {
		if err := client.DeclareQueue(queue); err != nil {
			return err
		}
		log.Printf("Declared queue: %s", queue)
	}

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
		{"warehouse.test", "warehouse", "warehouse.test"},
	}

	for _, binding := range bindings {
		if err := client.BindQueue(binding.queue, binding.exchange, binding.routingKey); err != nil {
			return err
		}
		log.Printf("Bound queue %s to exchange %s with key %s", binding.queue, binding.exchange, binding.routingKey)
	}

	return nil
}
