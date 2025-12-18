package main

import (
	"fmt"
	"os"

	onnxruntime "github.com/yalue/onnxruntime_go"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Usage: ./onnx-info <model_path>")
		os.Exit(1)
	}
	modelPath := os.Args[1]

	onnxruntime.SetSharedLibraryPath("third_party/lib/libonnxruntime.so")
	err := onnxruntime.InitializeEnvironment()
	if err != nil {
		fmt.Printf("❌ Failed to init ORT: %v\n", err)
		os.Exit(1)
	}
	defer onnxruntime.DestroyEnvironment()

	data, err := os.ReadFile(modelPath)
	if err != nil {
		fmt.Printf("❌ Failed to read file: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("🔍 Probing V4 shapes for %s\n", modelPath)

	tInput, _ := onnxruntime.NewTensor(onnxruntime.NewShape(1, 512), make([]float32, 512))
	tSR, _ := onnxruntime.NewTensor(onnxruntime.NewShape(1), []int64{16000})
	tOutput, _ := onnxruntime.NewTensor(onnxruntime.NewShape(1, 1), make([]float32, 1))

	shapes := [][]int64{
		{1, 1, 64},
		{2, 1, 64}, // The error suggested Expected 2 for index 0
		{1, 2, 64},
	}

	for _, s := range shapes {
		fmt.Printf("Testing shape %v for h/c...\n", s)
		size := int64(1)
		for _, d := range s {
			size *= d
		}

		tHIn, _ := onnxruntime.NewTensor(onnxruntime.NewShape(s...), make([]float32, size))
		tCIn, _ := onnxruntime.NewTensor(onnxruntime.NewShape(s...), make([]float32, size))
		tHOut, _ := onnxruntime.NewTensor(onnxruntime.NewShape(s...), make([]float32, size))
		tCOut, _ := onnxruntime.NewTensor(onnxruntime.NewShape(s...), make([]float32, size))

		session, err := onnxruntime.NewAdvancedSessionWithONNXData(
			data,
			[]string{"input", "sr", "h", "c"},
			[]string{"output", "hn", "cn"},
			[]onnxruntime.Value{tInput, tSR, tHIn, tCIn},
			[]onnxruntime.Value{tOutput, tHOut, tCOut},
			nil,
		)
		if err != nil {
			fmt.Printf("  Init error: %v\n", err)
			continue
		}

		err = session.Run()
		if err != nil {
			fmt.Printf("  Run error: %v\n", err)
		} else {
			fmt.Printf("  ✅ SUCCESS with shape %v\n", s)
		}
		session.Destroy()
		tHIn.Destroy()
		tCIn.Destroy()
		tHOut.Destroy()
		tCOut.Destroy()
	}
}
