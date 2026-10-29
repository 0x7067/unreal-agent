package workerruntime

import (
	"os"
	"testing"

	"github.com/unreallabsai/unreal-agent/internal/testtmp"
)

func TestMain(m *testing.M) {
	testtmp.Resolve()
	os.Exit(m.Run())
}
