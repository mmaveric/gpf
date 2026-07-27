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
