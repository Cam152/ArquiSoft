package commands

import (
	"fmt"
	"math"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"agora-admin/internal/client"
)

// ---------- Candidatos ----------

func (a *app) candidatesCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "candidates",
		Aliases: []string{"candidate", "c"},
		Short:   "Agrega o quita candidatos de una elección en borrador",
	}
	cmd.AddCommand(a.candidatesAdd(), a.candidatesRemove())
	return cmd
}

func (a *app) candidatesAdd() *cobra.Command {
	var (
		electionID        int
		name, description string
	)
	cmd := &cobra.Command{
		Use:   "add --election <id> --name <nombre>",
		Short: "Agrega un candidato",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if err := requirePositive("election", electionID); err != nil {
				return err
			}
			in := client.CandidateInput{Name: strings.TrimSpace(name)}
			if cmd.Flags().Changed("description") {
				in.Description = &description
			}
			c, err := a.api.AddCandidate(cmd.Context(), electionID, in)
			if err != nil {
				return err
			}
			if a.asJSON {
				return a.printJSON(cmd, c)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Candidato %d agregado a la elección %d: %s (tarjetón n.º %d)\n",
				c.ID, electionID, c.Name, c.Number)
			return nil
		},
	}
	cmd.Flags().IntVar(&electionID, "election", 0, "id de la elección")
	cmd.Flags().StringVar(&name, "name", "", "nombre del candidato")
	cmd.Flags().StringVar(&description, "description", "", "descripción o propuesta")
	_ = cmd.MarkFlagRequired("election")
	_ = cmd.MarkFlagRequired("name")
	return cmd
}

func (a *app) candidatesRemove() *cobra.Command {
	var electionID, candidateID int
	cmd := &cobra.Command{
		Use:   "remove --election <id> --candidate <id>",
		Short: "Quita un candidato",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if err := requirePositive("election", electionID); err != nil {
				return err
			}
			if err := requirePositive("candidate", candidateID); err != nil {
				return err
			}
			if err := a.api.RemoveCandidate(cmd.Context(), electionID, candidateID); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Candidato %d quitado de la elección %d.\n", candidateID, electionID)
			return nil
		},
	}
	cmd.Flags().IntVar(&electionID, "election", 0, "id de la elección")
	cmd.Flags().IntVar(&candidateID, "candidate", 0, "id del candidato")
	_ = cmd.MarkFlagRequired("election")
	_ = cmd.MarkFlagRequired("candidate")
	return cmd
}

// ---------- Resultados ----------

func (a *app) resultsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "results",
		Aliases: []string{"r"},
		Short:   "Consulta resultados",
	}
	cmd.AddCommand(&cobra.Command{
		Use:   "show <id>",
		Short: "Muestra el conteo de votos de una elección",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id, err := parseID("id", args[0])
			if err != nil {
				return err
			}
			r, err := a.api.Results(cmd.Context(), id)
			if err != nil {
				return err
			}
			if a.asJSON {
				return a.printJSON(cmd, r)
			}
			out := cmd.OutOrStdout()
			title := r.ElectionName
			if title == "" {
				title = fmt.Sprintf("Elección %d", r.ElectionID)
			}
			fmt.Fprintf(out, "%s\nTotal de votos: %d\n\n", title, r.TotalVotes)
			w := tabwriter.NewWriter(out, 0, 0, 2, ' ', 0)
			fmt.Fprintln(w, "CANDIDATO\tVOTOS\t%\t")
			for _, c := range r.Results {
				fmt.Fprintf(w, "%s\t%d\t%.2f\t%s\n", c.CandidateName, c.Votes, c.Percentage, bar(c.Percentage))
			}
			return w.Flush()
		},
	})
	return cmd
}

// bar dibuja el porcentaje con 20 bloques.
func bar(percentage float64) string {
	n := int(math.Round(math.Max(0, math.Min(100, percentage)) / 5))
	return strings.Repeat("█", n)
}

// ---------- Auditoría ----------

func (a *app) auditCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "audit",
		Short: "Consulta el registro de auditoría",
	}
	var electionID, limit int
	list := &cobra.Command{
		Use:   "list [--election <id>] [--limit 50]",
		Short: "Lista los eventos de auditoría más recientes",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if electionID < 0 {
				return fmt.Errorf("--election debe ser un entero positivo")
			}
			if limit < 1 || limit > 1000 {
				return fmt.Errorf("--limit debe estar entre 1 y 1000")
			}
			events, err := a.api.Audit(cmd.Context(), electionID, limit)
			if err != nil {
				return err
			}
			if a.asJSON {
				return a.printJSON(cmd, events)
			}
			if len(events) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "No hay eventos de auditoría.")
				return nil
			}
			w := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 2, ' ', 0)
			fmt.Fprintln(w, "EVENTO\tTIPO\tELECCIÓN\tOCURRIÓ\tREGISTRADO")
			for _, e := range events {
				fmt.Fprintf(w, "%s\t%s\t%d\t%s\t%s\n", e.EventID, e.Type, e.ElectionID, dash(e.OccurredAt), dash(e.RecordedAt))
			}
			return w.Flush()
		},
	}
	list.Flags().IntVar(&electionID, "election", 0, "filtra por elección")
	list.Flags().IntVar(&limit, "limit", 50, "máximo de eventos")
	cmd.AddCommand(list)
	return cmd
}

func dash(s string) string {
	if s == "" {
		return "-"
	}
	return s
}
