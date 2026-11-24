package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"golang.org/x/crypto/bcrypt"

	"github.com/example/be_jwt_golang/backend/internal/auth"
	"github.com/example/be_jwt_golang/backend/internal/branch"
	"github.com/example/be_jwt_golang/backend/internal/branchauth"
	"github.com/example/be_jwt_golang/backend/internal/config"
	"github.com/example/be_jwt_golang/backend/internal/db"
	"github.com/example/be_jwt_golang/backend/internal/distributor"
	"github.com/example/be_jwt_golang/backend/internal/docs"
	"github.com/example/be_jwt_golang/backend/internal/dokter"
	"github.com/example/be_jwt_golang/backend/internal/doktermaster"
	"github.com/example/be_jwt_golang/backend/internal/doktermcu"
	"github.com/example/be_jwt_golang/backend/internal/icd"
	"github.com/example/be_jwt_golang/backend/internal/jamkes"
	"github.com/example/be_jwt_golang/backend/internal/lab"
	"github.com/example/be_jwt_golang/backend/internal/labmaster"
	"github.com/example/be_jwt_golang/backend/internal/labprice"
	"github.com/example/be_jwt_golang/backend/internal/labsubmaster"
	"github.com/example/be_jwt_golang/backend/internal/mastermcu"
	"github.com/example/be_jwt_golang/backend/internal/menu"
	"github.com/example/be_jwt_golang/backend/internal/nonlab"
	"github.com/example/be_jwt_golang/backend/internal/obat"
	"github.com/example/be_jwt_golang/backend/internal/orglokasi"
	"github.com/example/be_jwt_golang/backend/internal/paketprice"
	"github.com/example/be_jwt_golang/backend/internal/pasien"
	"github.com/example/be_jwt_golang/backend/internal/pasienmcu"
	"github.com/example/be_jwt_golang/backend/internal/pendaftaran"
	"github.com/example/be_jwt_golang/backend/internal/penerimaan"
	"github.com/example/be_jwt_golang/backend/internal/pengobatan"
	"github.com/example/be_jwt_golang/backend/internal/penunjang"
	"github.com/example/be_jwt_golang/backend/internal/perujukotc"
	"github.com/example/be_jwt_golang/backend/internal/perusahaan"
	"github.com/example/be_jwt_golang/backend/internal/principal"
	"github.com/example/be_jwt_golang/backend/internal/reportklinik"
	"github.com/example/be_jwt_golang/backend/internal/role"
	"github.com/example/be_jwt_golang/backend/internal/tindakan"
	"github.com/example/be_jwt_golang/backend/internal/user"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("config: %v", err)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	pool, err := db.Connect(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("db: %v", err)
	}
	defer pool.Close()
	if err := db.Migrate(ctx, pool); err != nil {
		log.Fatalf("migrate: %v", err)
	}
	if cfg.SeedDemoUsers {
		if err := db.SeedDemoUsers(ctx, pool, bcrypt.DefaultCost); err != nil {
			log.Fatalf("seed: %v", err)
		}
	}

	tokens := auth.NewTokenManager(cfg.JWTSecret, cfg.JWTTTL)
	handler := auth.NewHandler(user.NewPostgresRepository(pool), tokens, bcrypt.DefaultCost)

	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("ok"))
	})
	handler.Routes(mux)
	menuRepo := menu.NewPostgresRepository(pool)
	menuHandler := menu.NewHandler(menuRepo)
	menuHandler.Routes(mux, tokens.Middleware)
	role.NewHandler(role.NewPostgresRepository(pool), menuRepo).Routes(mux, tokens.Middleware, menuHandler.RequireMenu)
	branch.NewHandler(branch.NewPostgresRepository(pool)).Routes(mux, tokens.Middleware, menuHandler.RequireMenu)
	mux.Handle("GET /api/me/branches", tokens.Middleware(branchauth.MineHandler(func(ctx context.Context, userID string) ([]branchauth.Branch, error) {
		return branchauth.ForUser(ctx, pool, userID)
	})))
	obat.NewHandler(obat.NewPostgresRepository(pool)).Routes(mux, tokens.Middleware, menuHandler.RequireMenu)
	tindakan.NewHandler(tindakan.NewPostgresRepository(pool)).Routes(mux, tokens.Middleware, menuHandler.RequireMenu)
	penunjang.NewHandler(penunjang.NewPostgresRepository(pool)).Routes(mux, tokens.Middleware, menuHandler.RequireMenu)
	pasien.NewHandler(pasien.NewPostgresRepository(pool)).Routes(mux, tokens.Middleware, menuHandler.RequireMenu)
	dokter.NewHandler(dokter.NewPostgresRepository(pool)).Routes(mux, tokens.Middleware, menuHandler.RequireMenu)
	jamkes.NewHandler(jamkes.NewPostgresRepository(pool)).Routes(mux, tokens.Middleware, menuHandler.RequireMenu)
	distributor.NewHandler(distributor.NewPostgresRepository(pool)).Routes(mux, tokens.Middleware, menuHandler.RequireMenu)
	principal.NewHandler(principal.NewPostgresRepository(pool)).Routes(mux, tokens.Middleware, menuHandler.RequireMenu)
	icd.NewHandler(icd.NewPostgresRepository(pool)).Routes(mux, tokens.Middleware, menuHandler.RequireMenu)
	lab.NewHandler(lab.NewPostgresRepository(pool)).Routes(mux, tokens.Middleware, menuHandler.RequireMenu)
	paketprice.NewHandler(paketprice.NewPostgresRepository(pool)).Routes(mux, tokens.Middleware, menuHandler.RequireMenu)
	labprice.NewHandler(labprice.NewPostgresRepository(pool)).Routes(mux, tokens.Middleware, menuHandler.RequireMenu)
	labmaster.NewHandler(labmaster.NewPostgresRepository(pool)).Routes(mux, tokens.Middleware, menuHandler.RequireMenu)
	labsubmaster.NewHandler(labsubmaster.NewPostgresRepository(pool)).Routes(mux, tokens.Middleware, menuHandler.RequireMenu)
	nonlab.NewHandler(nonlab.NewPostgresRepository(pool)).Routes(mux, tokens.Middleware, menuHandler.RequireMenu)
	doktermcu.NewHandler(doktermcu.NewPostgresRepository(pool)).Routes(mux, tokens.Middleware, menuHandler.RequireMenu)
	doktermaster.NewHandler(doktermaster.NewPostgresRepository(pool)).Routes(mux, tokens.Middleware, menuHandler.RequireMenu)
	mastermcu.NewHandler(mastermcu.NewPostgresRepository(pool)).Routes(mux, tokens.Middleware, menuHandler.RequireMenu)
	pasienmcu.NewHandler(pasienmcu.NewPostgresRepository(pool)).Routes(mux, tokens.Middleware, menuHandler.RequireMenu)
	perusahaan.NewHandler(perusahaan.NewPostgresRepository(pool)).Routes(mux, tokens.Middleware, menuHandler.RequireMenu)
	perujukotc.NewHandler(perujukotc.NewPostgresRepository(pool)).Routes(mux, tokens.Middleware, menuHandler.RequireMenu)
	pendaftaran.NewHandler(pendaftaran.NewPostgresRepository(pool)).Routes(mux, tokens.Middleware, menuHandler.RequireMenu)
	orglokasi.NewHandler(orglokasi.NewPostgresRepository(pool)).Routes(mux, tokens.Middleware, menuHandler.RequireMenu)
	pengobatan.NewHandler(pengobatan.NewPostgresRepository(pool)).Routes(mux, tokens.Middleware, menuHandler.RequireMenu)
	reportklinik.NewHandler(reportklinik.NewPostgresRepository(pool)).Routes(mux, tokens.Middleware, menuHandler.RequireMenu)
	penerimaan.NewHandler(penerimaan.NewPostgresRepository(pool)).Routes(mux, tokens.Middleware, menuHandler.RequireMenu)
	if cfg.SwaggerEnabled {
		docs.Register(mux)
		log.Printf("swagger UI enabled at /swagger/")
	}

	srv := &http.Server{
		Addr:              ":" + cfg.Port,
		Handler:           logRequests(mux),
		ReadHeaderTimeout: 5 * time.Second,
	}

	go func() {
		log.Printf("listening on :%s (token TTL %s)", cfg.Port, cfg.JWTTTL)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("server: %v", err)
		}
	}()

	<-ctx.Done()
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_ = srv.Shutdown(shutdownCtx)
	log.Println("server stopped")
}

func logRequests(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		next.ServeHTTP(w, r)
		log.Printf("%s %s %s", r.Method, r.URL.Path, time.Since(start))
	})
}
