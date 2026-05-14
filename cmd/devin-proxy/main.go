package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/ggwpgoend/devin-proxy/internal/admin"
	"github.com/ggwpgoend/devin-proxy/internal/crypto"
	"github.com/ggwpgoend/devin-proxy/internal/keypool"
	"github.com/ggwpgoend/devin-proxy/internal/proxy"
	"github.com/ggwpgoend/devin-proxy/internal/store"
)

func main() {
	proxyPort := flag.Int("proxy-port", 9090, "port for the reverse proxy")
	adminPort := flag.Int("admin-port", 9091, "port for the admin dashboard")
	dataDir := flag.String("data-dir", ".", "directory for database and master key")
	cooldownMin := flag.Int("cooldown", 10, "cooldown duration in minutes for exhausted keys")
	flag.Parse()

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	dbPath := filepath.Join(*dataDir, "devin-proxy.db")
	keyPath := filepath.Join(*dataDir, ".master_key")

	cipher, err := crypto.LoadOrCreateCipher(keyPath)
	if err != nil {
		log.Fatalf("crypto init: %v", err)
	}

	db, err := store.Open(ctx, dbPath)
	if err != nil {
		log.Fatalf("store init: %v", err)
	}
	defer db.Close()

	pool := keypool.New(db, cipher, time.Duration(*cooldownMin)*time.Minute)

	go proxy.StartReactivator(ctx, pool)

	proxyHandler := proxy.NewHandler(pool)
	proxySrv := &http.Server{
		Addr:         fmt.Sprintf(":%d", *proxyPort),
		Handler:      proxyHandler,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 120 * time.Second,
	}

	adminSrv := admin.NewServer(pool)
	adminHTTP := &http.Server{
		Addr:         fmt.Sprintf(":%d", *adminPort),
		Handler:      adminSrv.Router(),
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 30 * time.Second,
	}

	go func() {
		log.Printf("🔀 Proxy listening on http://localhost:%d → https://api.devin.ai", *proxyPort)
		if err := proxySrv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("proxy server: %v", err)
		}
	}()

	go func() {
		log.Printf("🛠  Admin panel on http://localhost:%d", *adminPort)
		if err := adminHTTP.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("admin server: %v", err)
		}
	}()

	<-ctx.Done()
	log.Println("shutting down...")

	shutCtx, shutCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutCancel()
	_ = proxySrv.Shutdown(shutCtx)
	_ = adminHTTP.Shutdown(shutCtx)
}
