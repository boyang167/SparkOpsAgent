package install

import "context"

type MemoryPublisher struct {
	Last PublishRequest
}

func (p *MemoryPublisher) Publish(ctx context.Context, req PublishRequest) (string, error) {
	_ = ctx
	p.Last = clonePublishRequest(req)
	return "https://example.local/pr/1", nil
}

func clonePublishRequest(req PublishRequest) PublishRequest {
	cloned := PublishRequest{
		Branch:  req.Branch,
		Message: req.Message,
	}
	if req.Files != nil {
		cloned.Files = make(map[string][]byte, len(req.Files))
		for name, content := range req.Files {
			cloned.Files[name] = append([]byte(nil), content...)
		}
	}
	if req.Delete != nil {
		cloned.Delete = append([]string(nil), req.Delete...)
	}
	return cloned
}
