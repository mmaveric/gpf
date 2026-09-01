package queue_test

import (
	"testing"

	"github.com/asharipov/gpf/batch"
	"github.com/asharipov/gpf/queue"
)

func TestQueueOpsDeclaration(t *testing.T) {
	t.Parallel()

	operations := queue.QueueOps[int]{
		TryPush: func(batch.Batch[int]) bool { return true },
		TryPop:  func(*batch.Batch[int]) bool { return false },
		Len:     func() uint32 { return 0 },
	}
	if !operations.TryPush(batch.Batch[int]{}) || operations.TryPop(&batch.Batch[int]{}) {
		t.Fatal("QueueOps callbacks were not preserved")
	}
}

func TestFactoryDeclaration(t *testing.T) {
	t.Parallel()

	var factory queue.Factory[int] = func(config queue.Config) (queue.QueueOps[int], error) {
		if config.Capacity != 1 || config.Policy != queue.Drop {
			t.Fatalf("factory config = %#v", config)
		}
		return queue.QueueOps[int]{}, nil
	}
	if _, err := factory(queue.Config{Capacity: 1, BatchCapacity: 1, Policy: queue.Drop}); err != nil {
		t.Fatalf("Factory() error = %v", err)
	}
}
