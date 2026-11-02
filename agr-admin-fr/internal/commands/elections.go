package commands

import (
	"fmt"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"

	"agora-admin/internal/client"
)

var statusNames = map[string]string{"draft": "borrador", "active": "activa", "closed": "cerrada"}

func statusName(s string) string {
	if name, ok := statusNames[s]; ok {
		return name
	}
	return s
}

func yesNo(b bool) string {
	if b {
		return "sí"
	}
	return "no"
}

func formatTime(t *time.Time) string {
	if t == nil {
		return "-"
	}
	return t.Format(time.RFC3339)
}

func (a *app) electionsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "elections",
		Aliases: []string{"election", "e"},
		Short:   "Lista, crea, edita, abre, cierra y borra elecciones",
	}
	cmd.AddCommand(
		a.electionsList(), a.electionsShow(), a.electionsCreate(), a.electionsUpdate(),
		a.electionsDelete(), a.electionsOpen(), a.electionsClose(),
	)
	return cmd
}

func (a *app) electionsList() *cobra.Command {
	var status string
	cmd := &cobra.Command{
		Use:   "list",
		Short: "Lista las elecciones",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if status != "" {
				if _, ok := statusNames[status]; !ok {
					return fmt.Errorf("--status debe ser draft, active o closed")
				}
			}
			elections, err := a.api.ListElections(cmd.Context(), status)
			if err != nil {
				return err
			}
			if a.asJSON {
				return a.printJSON(cmd, elections)
			}
			if len(elections) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "No hay elecciones.")
				return nil
			}
			w := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 2, ' ', 0)
			fmt.Fprintln(w, "ID\tNOMBRE\tESTADO\tABIERTA\tCANDIDATOS")
			for _, e := range elections {
				fmt.Fprintf(w, "%d\t%s\t%s\t%s\t%d\n", e.ID, e.Name, statusName(e.Status), yesNo(e.IsOpen), e.CandidateCount)
			}
			return w.Flush()
		},
	}
	cmd.Flags().StringVar(&status, "status", "", "filtra por estado: draft, active o closed")
	return cmd
}

func (a *app) electionsShow() *cobra.Command {
	return &cobra.Command{
		Use:   "show <id>",
		Short: "Muestra una elección con sus candidatos",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id, err := parseID("id", args[0])
			if err != nil {
				return err
			}
			e, err := a.api.GetElection(cmd.Context(), id)
			if err != nil {
				return err
			}
			if a.asJSON {
				return a.printJSON(cmd, e)
			}
			return printElection(cmd, e)
		},
	}
}

func printElection(cmd *cobra.Command, e *client.Election) error {
	w := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 2, ' ', 0)
	fmt.Fprintf(w, "ID:\t%d\n", e.ID)
	fmt.Fprintf(w, "Nombre:\t%s\n", e.Name)
	if e.Description != nil && *e.Description != "" {
		fmt.Fprintf(w, "Descripción:\t%s\n", *e.Description)
	}
	fmt.Fprintf(w, "Estado:\t%s (abierta para votar: %s)\n", statusName(e.Status), yesNo(e.IsOpen))
	fmt.Fprintf(w, "Inicio:\t%s\n", formatTime(e.StartsAt))
	fmt.Fprintf(w, "Fin:\t%s\n", formatTime(e.EndsAt))
	if err := w.Flush(); err != nil {
		return err
	}

	out := cmd.OutOrStdout()
	if len(e.Candidates) == 0 {
		fmt.Fprintln(out, "\nSin candidatos.")
		return nil
	}
	fmt.Fprintln(out)
	w = tabwriter.NewWriter(out, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "N.º\tID\tCANDIDATO")
	for _, c := range e.Candidates {
		fmt.Fprintf(w, "%d\t%d\t%s\n", c.Number, c.ID, c.Name)
	}
	return w.Flush()
}

// electionFlags son las banderas comunes de create y update.
type electionFlags struct {
	name, description, starts, ends string
}

func (f *electionFlags) register(cmd *cobra.Command) {
	cmd.Flags().StringVar(&f.name, "name", "", "nombre de la elección")
	cmd.Flags().StringVar(&f.description, "description", "", "descripción")
	cmd.Flags().StringVar(&f.starts, "starts", "", "inicio, p. ej. 2026-11-03T08:00:00-05:00")
	cmd.Flags().StringVar(&f.ends, "ends", "", "fin, p. ej. 2026-11-03T18:00:00-05:00")
}

// input arma el cuerpo con las banderas que el usuario escribió.
func (f *electionFlags) input(cmd *cobra.Command) (client.ElectionInput, error) {
	var in client.ElectionInput
	changed := cmd.Flags().Changed
	if changed("name") {
		name := strings.TrimSpace(f.name)
		in.Name = &name
	}
	if changed("description") {
		in.Description = &f.description
	}
	var err error
	if changed("starts") {
		if in.StartsAt, err = parseTime("starts", f.starts); err != nil {
			return in, err
		}
	}
	if changed("ends") {
		if in.EndsAt, err = parseTime("ends", f.ends); err != nil {
			return in, err
		}
	}
	return in, nil
}

func (a *app) electionsCreate() *cobra.Command {
	var f electionFlags
	cmd := &cobra.Command{
		Use:   "create --name <nombre>",
		Short: "Crea una elección en borrador",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			in, err := f.input(cmd)
			if err != nil {
				return err
			}
			e, err := a.api.CreateElection(cmd.Context(), in)
			if err != nil {
				return err
			}
			if a.asJSON {
				return a.printJSON(cmd, e)
			}
			out := cmd.OutOrStdout()
			fmt.Fprintf(out, "Elección %d creada en borrador: %s\n", e.ID, e.Name)
			fmt.Fprintf(out, "Siguiente paso: agora candidates add --election %d --name \"...\" (mínimo 2) y agora elections open %d\n", e.ID, e.ID)
			return nil
		},
	}
	f.register(cmd)
	_ = cmd.MarkFlagRequired("name")
	return cmd
}

func (a *app) electionsUpdate() *cobra.Command {
	var f electionFlags
	cmd := &cobra.Command{
		Use:   "update <id> [--name ...] [--description ...] [--starts ...] [--ends ...]",
		Short: "Edita una elección (solo los campos indicados)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id, err := parseID("id", args[0])
			if err != nil {
				return err
			}
			in, err := f.input(cmd)
			if err != nil {
				return err
			}
			if in == (client.ElectionInput{}) {
				return fmt.Errorf("indique al menos un campo: --name, --description, --starts o --ends")
			}
			e, err := a.api.UpdateElection(cmd.Context(), id, in)
			if err != nil {
				return err
			}
			if a.asJSON {
				return a.printJSON(cmd, e)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Elección %d actualizada.\n\n", e.ID)
			return printElection(cmd, e)
		},
	}
	f.register(cmd)
	return cmd
}

func (a *app) electionsDelete() *cobra.Command {
	var yes bool
	cmd := &cobra.Command{
		Use:   "delete <id>",
		Short: "Borra una elección y sus candidatos",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id, err := parseID("id", args[0])
			if err != nil {
				return err
			}
			if !yes {
				ok, err := a.confirm(cmd, fmt.Sprintf("¿Borrar la elección %d y todos sus candidatos?", id))
				if err != nil {
					return err
				}
				if !ok {
					fmt.Fprintln(cmd.OutOrStdout(), "Cancelado.")
					return nil
				}
			}
			if err := a.api.DeleteElection(cmd.Context(), id); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Elección %d borrada.\n", id)
			return nil
		},
	}
	cmd.Flags().BoolVarP(&yes, "yes", "y", false, "no pide confirmación")
	return cmd
}

func (a *app) electionsOpen() *cobra.Command {
	return a.transition("open <id>", "Abre la votación (borrador → activa; requiere 2 candidatos)", "abierta",
		func(cmd *cobra.Command, id int) (*client.Election, error) {
			return a.api.OpenElection(cmd.Context(), id)
		})
}

func (a *app) electionsClose() *cobra.Command {
	return a.transition("close <id>", "Cierra la votación (activa → cerrada)", "cerrada",
		func(cmd *cobra.Command, id int) (*client.Election, error) {
			return a.api.CloseElection(cmd.Context(), id)
		})
}

// transition arma open y close, que solo cambian la ruta y el mensaje.
func (a *app) transition(use, short, done string, call func(*cobra.Command, int) (*client.Election, error)) *cobra.Command {
	return &cobra.Command{
		Use:   use,
		Short: short,
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id, err := parseID("id", args[0])
			if err != nil {
				return err
			}
			e, err := call(cmd, id)
			if err != nil {
				return err
			}
			if a.asJSON {
				return a.printJSON(cmd, e)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Elección %d %s: %s (estado: %s)\n", e.ID, done, e.Name, statusName(e.Status))
			return nil
		},
	}
}
