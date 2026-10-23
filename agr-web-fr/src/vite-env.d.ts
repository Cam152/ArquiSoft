/// <reference types="vite/client" />

interface ImportMetaEnv {
  readonly VITE_ELECTION_API_URL?: string;
  readonly VITE_VOTER_API_URL?: string;
  readonly VITE_VOTE_API_URL?: string;
}

interface ImportMeta {
  readonly env: ImportMetaEnv;
}
