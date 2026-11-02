// Package commands define los comandos de la CLI "agora". Cada comando lee sus
// argumentos, llama a internal/client y muestra el resultado.
package commands

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"agora-admin/internal/client"
)

// Version la fija el Dockerfile al compilar (-ldflags).
var Version = "dev"

const (
	defaultGateway = "http://localhost:8080"
	defaultTimeout = 10 * time.Second
)

// app guarda las opciones globales y el cliente que usan todos los comandos.
type app struct {
	gateway string
	apiKey  string
	asJSON  bool
	timeout time.Duration

	api *client.Client
}

// NewRoot arma el comando raíz con todos los subcomandos.
func NewRoot() *cobra.Command {
	a := &app{}
	root := &cobra.Command{
		Use:   "agora",
		Short: "Consola de administración de AGORA",
		Long: `agora administra las elecciones de AGORA a través de agr-gateway-or.

Crea, edita, abre y cierra elecciones, gestiona candidatos y consulta resultados
y auditoría. Las operaciones de escritura usan la llave de administración.

Configuración (variables de entorno o banderas):
  AGORA_GATEWAY_URL   URL del gateway (por defecto ` + defaultGateway + `)
  AGORA_ADMIN_KEY     llave de administración (X-API-Key)`,
		Version:       Version,
		SilenceUsage:  true,
		SilenceErrors: true,
		PersistentPreRunE: func(cmd *cobra.Command, _ []string) error {
			if a.timeout <= 0 {
				return fmt.Errorf("--timeout debe ser positivo")
			}
			a.api = client.New(a.gateway, a.apiKey, a.timeout)
			return nil
		},
	}
	root.CompletionOptions.HiddenDefaultCmd = true

	flags := root.PersistentFlags()
	flags.StringVar(&a.gateway, "gateway", envOr("AGORA_GATEWAY_URL", defaultGateway), "URL del gateway")
	flags.StringVar(&a.apiKey, "api-key", os.Getenv("AGORA_ADMIN_KEY"), "llave de administración (mejor por AGORA_ADMIN_KEY)")
	flags.BoolVar(&a.asJSON, "json", false, "muestra la respuesta en JSON")
	flags.DurationVar(&a.timeout, "timeout", defaultTimeout, "tiempo máximo de cada petición")

	root.AddCommand(a.electionsCmd(), a.candidatesCmd(), a.resultsCmd(), a.auditCmd())
	return root
}

// Execute corre la CLI y devuelve el código de salida.
func Execute(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	root := NewRoot()
	root.SetArgs(args)
	root.SetOut(stdout)
	root.SetErr(stderr)
	if err := root.ExecuteContext(ctx); err != nil {
		fmt.Fprintf(stderr, "Error: %v\n", err)
		return 1
	}
	return 0
}

func envOr(key, fallback string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}
	return fallback
}

// ---------- Ayudas comunes ----------

// parseID lee un id positivo de un argumento.
func parseID(name, raw string) (int, error) {
	id, err := strconv.Atoi(raw)
	if err != nil || id <= 0 {
		return 0, fmt.Errorf("%s debe ser un entero positivo, se recibió %q", name, raw)
	}
	return id, nil
}

// requirePositive valida una bandera obligatoria con id.
func requirePositive(name string, value int) error {
	if value <= 0 {
		return fmt.Errorf("--%s es obligatorio y debe ser un entero positivo", name)
	}
	return nil
}

// parseTime acepta RFC 3339 con zona horaria, por ejemplo 2026-11-03T08:00:00-05:00.
func parseTime(name, raw string) (*time.Time, error) {
	t, err := time.Parse(time.RFC3339, raw)
	if err != nil {
		return nil, fmt.Errorf("--%s debe tener el formato 2026-11-03T08:00:00-05:00", name)
	}
	return &t, nil
}

func (a *app) printJSON(cmd *cobra.Command, v any) error {
	enc := json.NewEncoder(cmd.OutOrStdout())
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}

// confirm pregunta s/N en la terminal.
func (a *app) confirm(cmd *cobra.Command, question string) (bool, error) {
	fmt.Fprintf(cmd.OutOrStdout(), "%s [s/N]: ", question)
	line, err := bufio.NewReader(cmd.InOrStdin()).ReadString('\n')
	if err != nil && line == "" {
		return false, fmt.Errorf("no se pudo leer la confirmación; use --yes para no preguntar")
	}
	answer := strings.ToLower(strings.TrimSpace(line))
	return answer == "s" || answer == "si" || answer == "sí" || answer == "y" || answer == "yes", nil
}
