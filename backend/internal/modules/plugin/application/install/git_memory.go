package install

import "context"

type MemoryPublisher struct {
	Last         PublishRequest
	PullRequests map[string]PullRequestView
	PublishURL   string
	PublishErr   error
	LookupErr    error
	PublishCount int
	LookupCount  int
}

func (p *MemoryPublisher) Publish(ctx context.Context, req PublishRequest) (string, error) {
	_ = ctx
	p.Last = clonePublishRequest(req)
	p.PublishCount++
	if p.PublishErr != nil {
		return "", p.PublishErr
	}
	url := p.PublishURL
	if url == "" {
		url = "https://example.local/pr/1"
	}
	if p.PullRequests == nil {
		p.PullRequests = make(map[string]PullRequestView)
	}
	p.PullRequests[req.Branch] = PullRequestView{URL: url, State: "OPEN", HeadRefName: req.Branch}
	return url, nil
}

func (p *MemoryPublisher) LookupPullRequest(ctx context.Context, branch string) (PullRequestView, error) {
	_ = ctx
	p.LookupCount++
	if p.LookupErr != nil {
		return PullRequestView{}, p.LookupErr
	}
	view, found := p.PullRequests[branch]
	if !found {
		return PullRequestView{}, ErrPullRequestNotFound
	}
	return validateRecoverablePRView(view, branch)
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
