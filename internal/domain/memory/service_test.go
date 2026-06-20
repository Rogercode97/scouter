package memory_test

import (
	"context"
	"testing"

	"github.com/Rogercode97/scouter/internal/domain/memory"
)

func TestAppService_Signatures(t *testing.T) {
	// This will fail to compile if NewAppService still requires distiller
	// or if PassiveDistill/DistillAndSave don't accept it.
	svc := memory.NewAppService(nil)
	
	// mock distiller
	var distiller memory.Distiller
	
	err := svc.PassiveDistill(context.Background(), "scouter", nil, distiller)
	if err != nil {
		t.Log(err)
	}
	
	err = svc.DistillAndSave(context.Background(), "scouter", 24, distiller)
	if err != nil {
		t.Log(err)
	}
}
