package cmd

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/mattduck/diffyduck/aligner"
	"github.com/mattduck/diffyduck/git"
	"github.com/mattduck/diffyduck/parser"
	"github.com/mattduck/diffyduck/v2/app"
	"github.com/mattduck/diffyduck/v2/internal"
	"github.com/mattduck/diffyduck/v2/models"
)

// RunPOC runs the proof of concept virtual viewport demo
func RunPOC() error {
	internal.Log("[STARTUP] RunPOC started")

	// Get git diff output
	input, err := getGitDiffForPOC()
	if err != nil {
		return fmt.Errorf("failed to get git diff: %v", err)
	}

	if input == "" {
		// Create a synthetic large diff for testing
		input = createSyntheticDiff()
	}

	// Parse the diff
	diffParser := parser.NewDiffParser()
	fileDiffs, err := diffParser.Parse(input)
	if err != nil {
		return fmt.Errorf("failed to parse diff: %v", err)
	}

	// Convert to models.FileWithLines format
	var filesWithLines []models.FileWithLines
	retriever := git.NewFileRetriever()
	diffAligner := aligner.NewDiffAligner()

	for _, fileDiff := range fileDiffs {
		oldFileInfo, err := retriever.GetFileInfo(fileDiff.OldPath, true)
		if err != nil {
			return fmt.Errorf("error getting old file info: %v", err)
		}

		newFileInfo, err := retriever.GetFileInfo(fileDiff.NewPath, false)
		if err != nil {
			return fmt.Errorf("error getting new file info: %v", err)
		}

		var alignedLines []aligner.AlignedLine
		isBinaryFile := oldFileInfo.Type == git.BinaryFile || newFileInfo.Type == git.BinaryFile
		if !isBinaryFile {
			alignedLines = diffAligner.AlignFile(oldFileInfo.Lines, newFileInfo.Lines, fileDiff.Hunks)
		}

		filesWithLines = append(filesWithLines, models.FileWithLines{
			FileDiff:     fileDiff,
			AlignedLines: alignedLines,
			OldFileType:  oldFileInfo.Type,
			NewFileType:  newFileInfo.Type,
		})
	}

	// For POC, always use large synthetic data to demonstrate performance
	// Comment out to test with real diff data
	// filesWithLines = createSyntheticFileData()
	// internal.Logf("[STARTUP] Created synthetic file data (%d files)", len(filesWithLines))

	// Use real diff data if available
	if len(filesWithLines) == 0 {
		filesWithLines = createSyntheticFileData()
		internal.Logf("[STARTUP] No real diff data, using synthetic data (%d files)", len(filesWithLines))
	} else {
		internal.Logf("[STARTUP] Using real diff data (%d files)", len(filesWithLines))
	}

	// Create and run the POC app
	internal.Log("[STARTUP] About to create POC app...")
	pocApp, err := app.NewPOCApp(filesWithLines)
	if err != nil {
		return fmt.Errorf("failed to create POC app: %v", err)
	}
	internal.Log("[STARTUP] POC app created, about to run...")

	return pocApp.Run()
}

// getGitDiffForPOC gets git diff output for the POC
func getGitDiffForPOC() (string, error) {
	// Check if stdin has data
	stat, _ := os.Stdin.Stat()
	if (stat.Mode() & os.ModeCharDevice) == 0 {
		// Data is being piped in
		data, err := io.ReadAll(os.Stdin)
		if err != nil {
			return "", err
		}
		return string(data), nil
	}

	// No stdin data, return empty to use synthetic data
	return "", nil
}

// createSyntheticDiff creates a large synthetic diff for performance testing
func createSyntheticDiff() string {
	var diff strings.Builder

	// Create a realistic Go file diff with many changes
	diff.WriteString(`diff --git a/large_file.go b/large_file.go
index 1234567..8901234 100644
--- a/large_file.go
+++ b/large_file.go
@@ -1,150 +1,200 @@
 package main

 import (
+	"context"
+	"encoding/json"
+	"errors"
 	"fmt"
+	"log"
+	"net/http"
 	"os"
+	"strings"
+	"sync"
 	"time"
 )

+// Config represents application configuration with enhanced options
+type Config struct {
+	Host        string        ` + "`json:\"host\"`" + `
+	Port        int           ` + "`json:\"port\"`" + `
+	Timeout     time.Duration ` + "`json:\"timeout\"`" + `
+	Debug       bool          ` + "`json:\"debug\"`" + `
+	Workers     int           ` + "`json:\"workers\"`" + `
+	MaxRequests int           ` + "`json:\"max_requests\"`" + `
+}

+// Server represents our enhanced application server
+type Server struct {
+	config    *Config
+	mu        sync.RWMutex
+	handlers  map[string]HandlerFunc
+	logger    *log.Logger
+	stats     *ServerStats
+	shutdown  chan struct{}
+}

+// ServerStats tracks server performance metrics
+type ServerStats struct {
+	RequestCount    int64
+	ErrorCount      int64
+	AverageResponse time.Duration
+	mu             sync.RWMutex
+}

+// HandlerFunc represents an enhanced request handler
+type HandlerFunc func(context.Context, *Request) (*Response, error)

 func main() {
-	fmt.Println("Hello World")
+	ctx, cancel := context.WithCancel(context.Background())
+	defer cancel()
+	
+	cfg := loadConfigFromFile("config.json")
+	if cfg == nil {
+		cfg = loadDefaultConfig()
+	}
+	
+	if cfg.Debug {
+		log.SetFlags(log.LstdFlags | log.Lshortfile)
+		log.Println("Debug mode enabled")
+	}
+	
+	server := NewServerWithStats(cfg)
+	log.Printf("Starting enhanced server on %s:%d with %d workers", cfg.Host, cfg.Port, cfg.Workers)
+	
+	// Setup graceful shutdown
+	go handleShutdownSignals(cancel)
+	
+	if err := server.RunWithGracefulShutdown(ctx); err != nil && !errors.Is(err, context.Canceled) {
+		log.Fatalf("Server failed: %v", err)
+	}
+	
+	log.Println("Server shutdown completed")
+}

-func processData(input string) string {
-	return strings.ToUpper(input)
+// NewServerWithStats creates a new server instance with performance monitoring
+func NewServerWithStats(cfg *Config) *Server {
+	return &Server{
+		config:   cfg,
+		handlers: make(map[string]HandlerFunc),
+		logger:   log.New(os.Stdout, "[SERVER] ", log.LstdFlags),
+		stats:    &ServerStats{},
+		shutdown: make(chan struct{}),
+	}
+}

-func calculateSum(a, b int) int {
-	return a + b
+// RunWithGracefulShutdown starts the server with enhanced shutdown handling
+func (s *Server) RunWithGracefulShutdown(ctx context.Context) error {
+	var wg sync.WaitGroup
+	
+	// Start worker goroutines with better error handling
+	for i := 0; i < s.config.Workers; i++ {
+		wg.Add(1)
+		go s.enhancedWorker(ctx, &wg, i)
+	}
+	
+	// Start metrics collector
+	wg.Add(1)
+	go s.metricsCollector(ctx, &wg)
+	
+	// Wait for context cancellation or shutdown signal
+	select {
+	case <-ctx.Done():
+		s.logger.Println("Context cancelled, initiating shutdown...")
+	case <-s.shutdown:
+		s.logger.Println("Shutdown signal received...")
+	}
+	
+	// Wait for all workers to finish with timeout
+	done := make(chan struct{})
+	go func() {
+		wg.Wait()
+		close(done)
+	}()
+	
+	select {
+	case <-done:
+		s.logger.Println("All workers finished gracefully")
+	case <-time.After(30 * time.Second):
+		s.logger.Println("Forced shutdown after timeout")
+	}
+	
+	return ctx.Err()
+}

+func (s *Server) enhancedWorker(ctx context.Context, wg *sync.WaitGroup, id int) {
+	defer wg.Done()
+	
+	s.logger.Printf("Enhanced worker %d started with improved error handling", id)
+	ticker := time.NewTicker(100 * time.Millisecond)
+	defer ticker.Stop()
+	
+	for {
+		select {
+		case <-ctx.Done():
+			s.logger.Printf("Worker %d shutting down gracefully", id)
+			return
+		case <-ticker.C:
+			// Process work items with error recovery
+			if err := s.processWorkItem(ctx, id); err != nil {
+				s.logger.Printf("Worker %d error: %v", id, err)
+				s.incrementErrorCount()
+			}
+		}
+	}
+}

+func processDataWithValidation(ctx context.Context, input string) (string, error) {
+	// Enhanced input validation and processing
+	if input == "" {
+		return "", fmt.Errorf("empty input provided")
+	}
+	
+	if len(input) > 10000 {
+		return "", fmt.Errorf("input too large: %d characters", len(input))
+	}
+	
+	select {
+	case <-ctx.Done():
+		return "", ctx.Err()
+	default:
+		// Process the data with advanced algorithms and logging
+		start := time.Now()
+		result := strings.ToUpper(strings.TrimSpace(input))
+		processed := fmt.Sprintf("PROCESSED[%s] at %v (took %v)", result, time.Now().Format(time.RFC3339), time.Since(start))
+		
+		log.Printf("Data processing completed: input_len=%d, output_len=%d, duration=%v", 
+			len(input), len(processed), time.Since(start))
+		
+		return processed, nil
+	}
+}

+func calculateSumWithErrorHandling(numbers ...int) (int, error) {
+	if len(numbers) == 0 {
+		return 0, fmt.Errorf("no numbers provided for calculation")
+	}
+	
+	if len(numbers) > 1000 {
+		return 0, fmt.Errorf("too many numbers: %d (max 1000)", len(numbers))
+	}
+	
+	sum := 0
+	for i, n := range numbers {
+		// Check for potential overflow
+		if sum > 0 && n > 0 && sum > int(^uint(0)>>1)-n {
+			return 0, fmt.Errorf("integer overflow at position %d", i)
+		}
+		sum += n
+	}
+	
+	log.Printf("Sum calculation completed: %d numbers, result=%d", len(numbers), sum)
+	return sum, nil
 }

 // Additional test functions with comprehensive improvements
 func testFunction1() {
-	fmt.Println("Test 1")
+	log.Println("Running enhanced test 1 with comprehensive logging and metrics collection")
+	start := time.Now()
+	
+	defer func() {
+		if r := recover(); r != nil {
+			log.Printf("Test 1 panic recovered: %v", r)
+		}
+		log.Printf("Test 1 completed in %v", time.Since(start))
+	}()
+	
+	// Simulate complex test logic
+	for i := 0; i < 100; i++ {
+		if err := validateComplexInput(fmt.Sprintf("test-data-%d", i)); err != nil {
+			log.Printf("Validation failed for item %d: %v", i, err)
+		}
+	}
 }

 func testFunction2() {
-	fmt.Println("Test 2")
+	log.Println("Running enhanced test 2 with advanced performance monitoring and resource tracking")
+	
+	// Create a context with timeout for better resource management
+	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
+	defer cancel()
+	
+	var wg sync.WaitGroup
+	results := make(chan string, 10)
+	
+	// Start multiple test workers
+	for i := 0; i < 5; i++ {
+		wg.Add(1)
+		go func(workerID int) {
+			defer wg.Done()
+			
+			for j := 0; j < 20; j++ {
+				select {
+				case <-ctx.Done():
+					log.Printf("Worker %d interrupted by context", workerID)
+					return
+				default:
+					result := fmt.Sprintf("worker-%d-result-%d", workerID, j)
+					results <- result
+					time.Sleep(10 * time.Millisecond)
+				}
+			}
+		}(i)
+	}
+	
+	// Close results channel when all workers are done
+	go func() {
+		wg.Wait()
+		close(results)
+	}()
+	
+	// Collect and process results
+	var collectedResults []string
+	for result := range results {
+		collectedResults = append(collectedResults, result)
+	}
+	
+	log.Printf("Test 2 collected %d results from worker pool", len(collectedResults))
 }

 func testFunction3() {
-	fmt.Println("Test 3")
+	log.Println("Running enhanced test 3 with distributed processing and advanced error recovery")
+	
+	// Setup error recovery and metrics
+	defer func() {
+		if r := recover(); r != nil {
+			log.Printf("Test 3 recovered from panic: %v", r)
+		}
+	}()
+	
+	// Distributed processing with error handling
+	var mu sync.Mutex
+	var successCount, errorCount int
+	var wg sync.WaitGroup
+	
+	// Process items concurrently with error tracking
+	for i := 0; i < 50; i++ {
+		wg.Add(1)
+		go func(itemID int) {
+			defer wg.Done()
+			
+			// Simulate work that might fail
+			if err := processTestItem(itemID); err != nil {
+				mu.Lock()
+				errorCount++
+				mu.Unlock()
+				log.Printf("Failed to process item %d: %v", itemID, err)
+			} else {
+				mu.Lock()
+				successCount++
+				mu.Unlock()
+			}
+		}(i)
+	}
+	
+	wg.Wait()
+	
+	log.Printf("Test 3 completed: %d successes, %d errors", successCount, errorCount)
+	
+	// Calculate and log success rate
+	total := successCount + errorCount
+	if total > 0 {
+		successRate := float64(successCount) / float64(total) * 100
+		log.Printf("Test 3 success rate: %.2f%%", successRate)
+	}
 }
`)

	return diff.String()
}

// createSyntheticFileData creates synthetic file data for performance testing
func createSyntheticFileData() []models.FileWithLines {
	// Create a large synthetic file with many lines for testing
	var alignedLines []aligner.AlignedLine

	// Generate 2000 lines of synthetic diff content
	for i := 0; i < 2000; i++ {
		var lineType aligner.LineType
		var oldLine, newLine *string

		if i%10 == 0 {
			// Every 10th line is deleted
			lineType = aligner.Deleted
			content := fmt.Sprintf("// This is old line %d with some code content that makes it long enough to test horizontal scrolling functionality", i)
			oldLine = &content
		} else if i%10 == 1 {
			// Every 10th+1 line is added
			lineType = aligner.Added
			content := fmt.Sprintf("// This is new line %d with some updated code content that makes it long enough to test horizontal scrolling functionality", i)
			newLine = &content
		} else if i%10 == 2 {
			// Every 10th+2 line is modified
			lineType = aligner.Modified
			oldContent := fmt.Sprintf("func oldFunction%d() { return fmt.Sprintf(\"old implementation %%d\", %d) }", i, i)
			newContent := fmt.Sprintf("func newFunction%d() { return fmt.Sprintf(\"new implementation %%d\", %d) }", i, i)
			oldLine = &oldContent
			newLine = &newContent

			// Note: Word diff will be computed by the aligner when processing real diff data
			// For synthetic data, we're not computing word diff here
		} else {
			// Other lines are unchanged
			lineType = aligner.Unchanged
			content := fmt.Sprintf("    unchanged line %d with regular content that also needs to be long enough for horizontal scroll testing", i)
			oldLine = &content
			newLine = &content
		}

		alignedLines = append(alignedLines, aligner.AlignedLine{
			OldLine:    oldLine,
			NewLine:    newLine,
			LineType:   lineType,
			OldLineNum: i + 1,
			NewLineNum: i + 1,
		})
	}

	// Create a file diff
	fileDiff := parser.FileDiff{
		OldPath:   "test/large_file.go",
		NewPath:   "test/large_file.go",
		Additions: 500,
		Deletions: 300,
	}

	return []models.FileWithLines{
		{
			FileDiff:     fileDiff,
			AlignedLines: alignedLines,
			OldFileType:  git.TextFile,
			NewFileType:  git.TextFile,
		},
	}
}
