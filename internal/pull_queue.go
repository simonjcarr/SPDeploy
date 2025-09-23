package internal

import (
	"sync"
	"spdeploy/internal/logger"
	"go.uber.org/zap"
)

type PullRequest struct {
	Repo       Repository
	RepoLogger *logger.RepoLogger
}

type PullQueue struct {
	mu         sync.Mutex
	queue      []PullRequest
	processing bool
	cond       *sync.Cond
}

func NewPullQueue() *PullQueue {
	pq := &PullQueue{
		queue:      make([]PullRequest, 0),
		processing: false,
	}
	pq.cond = sync.NewCond(&pq.mu)
	return pq
}

func (pq *PullQueue) Add(repo Repository, repoLogger *logger.RepoLogger) {
	pq.mu.Lock()
	defer pq.mu.Unlock()

	for _, pr := range pq.queue {
		if pr.Repo.URL == repo.URL && pr.Repo.Branch == repo.Branch {
			logger.Info("Pull request already queued",
				zap.String("repo", repo.URL),
				zap.String("branch", repo.Branch))
			return
		}
	}

	pq.queue = append(pq.queue, PullRequest{
		Repo:       repo,
		RepoLogger: repoLogger,
	})

	logger.Info("Added pull request to queue",
		zap.String("repo", repo.URL),
		zap.String("branch", repo.Branch),
		zap.Int("queue_size", len(pq.queue)))

	pq.cond.Signal()
}

func (pq *PullQueue) IsProcessing() bool {
	pq.mu.Lock()
	defer pq.mu.Unlock()
	return pq.processing
}

func (pq *PullQueue) SetProcessing(processing bool) {
	pq.mu.Lock()
	defer pq.mu.Unlock()
	pq.processing = processing
	if !processing {
		pq.cond.Signal()
	}
}

func (pq *PullQueue) GetNext() (PullRequest, bool) {
	pq.mu.Lock()
	defer pq.mu.Unlock()

	for pq.processing && len(pq.queue) > 0 {
		pq.cond.Wait()
	}

	if len(pq.queue) == 0 {
		return PullRequest{}, false
	}

	next := pq.queue[0]
	pq.queue = pq.queue[1:]
	pq.processing = true

	return next, true
}

func (pq *PullQueue) Size() int {
	pq.mu.Lock()
	defer pq.mu.Unlock()
	return len(pq.queue)
}