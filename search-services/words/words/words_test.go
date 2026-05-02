package words

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNormalizePhrase(t *testing.T) {
	canceledCtx, cancel := context.WithCancel(context.Background())
	cancel()

	tests := []struct {
		name           string
		input          string
		expectedOutput []string
		ctx            context.Context
		errInReq       bool
	}{
		{
			name:           "Normalize words with simple phrase and context.Background",
			input:          "london loves the mystery of a speeding car",
			expectedOutput: []string{"london", "love", "mysteri", "speed", "car"},
			ctx:            context.Background(),
			errInReq:       false,
		},
		{
			name:           "Normalize words with simple phrase and canceled context",
			input:          "london loves the mystery of a speeding car",
			expectedOutput: nil,
			ctx:            canceledCtx,
			errInReq:       true,
		},
		{
			name:           "Normalize words with repeats words in phrase",
			input:          "linux was created in 1991. linus torvalds create linux",
			expectedOutput: []string{"linux", "creat", "1991", "linus", "torvald"},
			ctx:            context.Background(),
			errInReq:       false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			out, err := NormalizePhrase(tt.ctx, tt.input)

			if tt.errInReq == false {
				require.NoError(t, err)
			} else {
				require.Error(t, err)
				require.Equal(t, tt.expectedOutput, out)
				return
			}

			require.Equal(t, tt.expectedOutput, out)
		})
	}
}
