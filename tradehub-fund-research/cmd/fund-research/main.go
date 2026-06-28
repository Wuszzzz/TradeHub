package main

import (
	"database/sql"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	_ "github.com/lib/pq"

	"tradehub-fund-research/internal/research"
)

func main() {
	addr := getenv("FUND_RESEARCH_ADDR", ":17081")
	db, err := openDB()
	if err != nil {
		log.Printf("fund-research db unavailable: %v", err)
	}
	if db != nil {
		defer db.Close()
	}

	s := research.NewServer(db, research.NewEastMoneyClient(12*time.Second))
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", s.Health)
	mux.HandleFunc("GET /api/fund-research/v1/health", s.Health)
	mux.HandleFunc("GET /api/fund-research/v1/summary", s.Summary)
	mux.HandleFunc("GET /api/fund-research/v1/funds/4433", s.Fund4433)
	mux.HandleFunc("GET /api/fund-research/v1/funds/filter", s.FundFilter)
	mux.HandleFunc("POST /api/fund-research/v1/funds/check", s.FundCheck)
	mux.HandleFunc("POST /api/fund-research/v1/funds/similarity", s.FundSimilarity)
	mux.HandleFunc("GET /api/fund-research/v1/funds/by-stock", s.FundByStock)
	mux.HandleFunc("GET /api/fund-research/v1/managers", s.Managers)
	mux.HandleFunc("GET /api/fund-research/v1/sectors/related", s.RelatedSectors)
	mux.HandleFunc("GET /api/fund-research/v1/sectors/quotes", s.SectorQuotes)
	mux.HandleFunc("GET /api/fund-research/v1/tags/recommend", s.RecommendTags)
	mux.HandleFunc("GET /api/fund-research/v1/sync/status", s.SyncStatus)
	mux.HandleFunc("POST /api/fund-research/v1/sync/sector-map", s.SyncSectorMap)
	mux.HandleFunc("POST /api/fund-research/v1/sync/evaluations", s.SyncEvaluations)

	log.Printf("fund-research listening on %s", addr)
	if err := http.ListenAndServe(addr, loggingMiddleware(cors(mux))); err != nil {
		log.Fatal(err)
	}
}

func getenv(key, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(key)); value != "" {
		return value
	}
	return fallback
}

func openDB() (*sql.DB, error) {
	host := getenv("POSTGRES_HOST", "")
	if host == "" {
		return nil, nil
	}
	dsn := "host=" + host +
		" port=" + getenv("POSTGRES_PORT", "5432") +
		" user=" + getenv("POSTGRES_USER", "fundval") +
		" password=" + getenv("POSTGRES_PASSWORD", "") +
		" dbname=" + getenv("POSTGRES_DB", "fundval") +
		" sslmode=disable"
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		return nil, err
	}
	if err := db.Ping(); err != nil {
		_ = db.Close()
		return nil, err
	}
	return db, nil
}

func loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		next.ServeHTTP(w, r)
		log.Printf("%s %s %s", r.Method, r.URL.Path, time.Since(start))
	})
}

func cors(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET,POST,OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type,Authorization")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}
