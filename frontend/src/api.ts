// Cliente del Election Service (conector GraphQL sobre HTTP).
// Cuando existan el Voter Service y el Vote Service, sus llamadas REST
// se pueden agregar en este mismo archivo.

const ELECTION_API_URL: string =
  import.meta.env.VITE_ELECTION_API_URL ?? "http://localhost:8000";

export type ElectionStatus = "draft" | "active" | "closed";

export interface Candidate {
  id: number;
  number: number;
  name: string;
  description: string | null;
}

export interface Election {
  id: number;
  name: string;
  description: string | null;
  status: ElectionStatus;
  startsAt: string | null;
  endsAt: string | null;
  isOpen: boolean;
  candidateCount: number;
  candidates: Candidate[];
}

const ELECTIONS_QUERY = `
  query Elections {
    elections {
      id
      name
      description
      status
      startsAt
      endsAt
      isOpen
      candidateCount
      candidates { id number name description }
    }
  }
`;

export async function fetchElections(): Promise<Election[]> {
  const response = await fetch(`${ELECTION_API_URL}/graphql`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ query: ELECTIONS_QUERY }),
  });
  if (!response.ok) {
    throw new Error(`El Election Service respondió con HTTP ${response.status}`);
  }
  const body = await response.json();
  if (body.errors?.length) {
    throw new Error(body.errors[0].message);
  }
  return body.data.elections as Election[];
}
