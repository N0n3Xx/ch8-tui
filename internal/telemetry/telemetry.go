package telemetry

import "time"

type Telemetry struct {
	RequestStartedAt        time.Time `json:"request_started_at"`
	FirstTokenAt            time.Time `json:"first_token_at,omitempty"`
	LastTokenAt             time.Time `json:"last_token_at,omitempty"`
	RequestCompletedAt      time.Time `json:"request_completed_at"`
	Status                  string    `json:"status"`
	Model                   string    `json:"model"`
	TotalResponseTimeMillis int64     `json:"total_response_time_ms"`
	TimeToFirstTokenMillis  int64     `json:"time_to_first_token_ms"`
	TokensPerSecond         float64   `json:"tokens_per_second"`
	PromptTokens            int       `json:"prompt_tokens,omitempty"`
	CompletionTokens        int       `json:"completion_tokens,omitempty"`
	TotalTokens             int       `json:"total_tokens,omitempty"`
	TotalDurationNanos      int64     `json:"total_duration,omitempty"`
	LoadDurationNanos       int64     `json:"load_duration,omitempty"`
	PromptEvalDurationNanos int64     `json:"prompt_eval_duration,omitempty"`
	EvalDurationNanos       int64     `json:"eval_duration,omitempty"`
}

func FromOllama(model string, started, firstToken, lastToken, ended time.Time, status string, promptCount, evalCount int, totalDuration, loadDuration, promptEvalDuration, evalDuration int64) Telemetry {
	responseTime := ended.Sub(started)
	first := time.Duration(0)
	if !firstToken.IsZero() {
		first = firstToken.Sub(started)
	}
	tps := 0.0
	if evalCount > 0 {
		if evalDuration > 0 {
			tps = float64(evalCount) / (float64(evalDuration) / float64(time.Second))
		} else if responseTime > 0 {
			tps = float64(evalCount) / responseTime.Seconds()
		}
	}
	return Telemetry{
		RequestStartedAt:        started,
		FirstTokenAt:            firstToken,
		LastTokenAt:             lastToken,
		RequestCompletedAt:      ended,
		Status:                  status,
		Model:                   model,
		TotalResponseTimeMillis: responseTime.Milliseconds(),
		TimeToFirstTokenMillis:  first.Milliseconds(),
		TokensPerSecond:         tps,
		PromptTokens:            promptCount,
		CompletionTokens:        evalCount,
		TotalTokens:             promptCount + evalCount,
		TotalDurationNanos:      totalDuration,
		LoadDurationNanos:       loadDuration,
		PromptEvalDurationNanos: promptEvalDuration,
		EvalDurationNanos:       evalDuration,
	}
}

func (t Telemetry) ResponseTime() time.Duration {
	return time.Duration(t.TotalResponseTimeMillis) * time.Millisecond
}

func (t Telemetry) FirstTokenTime() time.Duration {
	return time.Duration(t.TimeToFirstTokenMillis) * time.Millisecond
}
