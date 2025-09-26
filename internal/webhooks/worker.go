package webhooks

import (
	"context"
	"log"
	"time"
)

const (
	WorkerBatchSize = 10
	WorkerInterval  = 10 * time.Second
)

// StartDeliveryWorker starts the background worker to process webhook deliveries
func (s *Service) StartDeliveryWorker(ctx context.Context) {
	log.Println("Starting webhook delivery worker...")

	ticker := time.NewTicker(WorkerInterval)
	defer ticker.Stop()

	// Process any pending deliveries immediately on startup
	if err := s.ProcessPendingDeliveries(ctx, WorkerBatchSize); err != nil {
		log.Printf("Error processing initial pending deliveries: %v", err)
	}

	for {
		select {
		case <-ctx.Done():
			log.Println("Webhook delivery worker shutting down...")
			return
		case <-ticker.C:
			if err := s.ProcessPendingDeliveries(ctx, WorkerBatchSize); err != nil {
				log.Printf("Error processing pending deliveries: %v", err)
			}
		}
	}
}

// ProcessAllPendingDeliveries processes all pending deliveries in batches
// This is useful for maintenance or catch-up scenarios
func (s *Service) ProcessAllPendingDeliveries(ctx context.Context) error {
	totalProcessed := 0
	batchSize := int32(WorkerBatchSize)

	for {
		deliveries, err := s.db.Queries.GetPendingWebhookDeliveries(ctx, batchSize)
		if err != nil {
			return err
		}

		if len(deliveries) == 0 {
			break // No more pending deliveries
		}

		log.Printf("Processing batch of %d webhook deliveries", len(deliveries))

		for _, delivery := range deliveries {
			if err := s.processDelivery(ctx, delivery); err != nil {
				log.Printf("Failed to process delivery %s: %v", delivery.ID, err)
			}
		}

		totalProcessed += len(deliveries)

		// If we got fewer than the batch size, we're done
		if len(deliveries) < int(batchSize) {
			break
		}

		// Small delay between batches to avoid overwhelming the database
		time.Sleep(100 * time.Millisecond)
	}

	log.Printf("Processed %d total webhook deliveries", totalProcessed)
	return nil
}
