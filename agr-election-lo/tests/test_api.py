ADMIN = {"X-API-Key": "test-key"}


def create_election(client, name="Elección de prueba", **extra):
    response = client.post("/elections", json={"name": name, **extra}, headers=ADMIN)
    assert response.status_code == 201, response.text
    return response.json()


def add_candidate(client, election_id, name):
    response = client.post(
        f"/elections/{election_id}/candidates", json={"name": name}, headers=ADMIN
    )
    assert response.status_code == 201, response.text
    return response.json()


def test_health(client):
    response = client.get("/health")
    assert response.status_code == 200
    assert response.json()["status"] == "ok"


def test_write_requires_api_key(client):
    assert client.post("/elections", json={"name": "Sin llave"}).status_code == 401
    bad = client.post("/elections", json={"name": "Llave mala"}, headers={"X-API-Key": "x"})
    assert bad.status_code == 401
    assert client.post("/elections", json={"name": "Con llave"}, headers=ADMIN).status_code == 201


def test_create_and_read(client):
    created = create_election(client, description="Descripción")
    assert created["status"] == "draft"
    assert created["is_open"] is False
    assert created["candidates"] == []

    detail = client.get(f"/elections/{created['id']}")
    assert detail.status_code == 200
    assert detail.json()["name"] == "Elección de prueba"

    listing = client.get("/elections")
    assert listing.status_code == 200
    assert len(listing.json()) == 1
    assert listing.json()[0]["candidate_count"] == 0


def test_not_found(client):
    assert client.get("/elections/999").status_code == 404
    assert client.post("/elections/999/activate", headers=ADMIN).status_code == 404


def test_validation_errors(client):
    short = client.post("/elections", json={"name": "ab"}, headers=ADMIN)
    assert short.status_code == 422
    bad_window = client.post(
        "/elections",
        json={
            "name": "Fechas malas",
            "starts_at": "2026-12-01T00:00:00Z",
            "ends_at": "2026-11-01T00:00:00Z",
        },
        headers=ADMIN,
    )
    assert bad_window.status_code == 422


def test_cannot_activate_without_enough_candidates(client):
    election = create_election(client)
    add_candidate(client, election["id"], "Único candidato")
    response = client.post(f"/elections/{election['id']}/activate", headers=ADMIN)
    assert response.status_code == 409


def test_full_lifecycle(client):
    election = create_election(client)
    eid = election["id"]
    first = add_candidate(client, eid, "Ana")
    second = add_candidate(client, eid, "Luis")
    assert (first["number"], second["number"]) == (1, 2)

    duplicate = client.post(
        f"/elections/{eid}/candidates", json={"name": "ana"}, headers=ADMIN
    )
    assert duplicate.status_code == 409

    opened = client.post(f"/elections/{eid}/activate", headers=ADMIN)
    assert opened.status_code == 200
    assert opened.json()["status"] == "active"
    assert opened.json()["is_open"] is True
    assert [c["name"] for c in opened.json()["candidates"]] == ["Ana", "Luis"]

    # Con la elección abierta ya no se puede modificar
    assert client.post(
        f"/elections/{eid}/candidates", json={"name": "Tarde"}, headers=ADMIN
    ).status_code == 409
    assert client.patch(f"/elections/{eid}", json={"name": "Nuevo nombre"}, headers=ADMIN).status_code == 409
    assert client.delete(f"/elections/{eid}", headers=ADMIN).status_code == 409
    assert client.post(f"/elections/{eid}/activate", headers=ADMIN).status_code == 409

    closed = client.post(f"/elections/{eid}/close", headers=ADMIN)
    assert closed.status_code == 200
    assert closed.json()["status"] == "closed"
    assert closed.json()["is_open"] is False
    assert client.post(f"/elections/{eid}/close", headers=ADMIN).status_code == 409


def test_filter_by_status(client):
    draft = create_election(client, name="Borrador")
    active = create_election(client, name="Activa")
    add_candidate(client, active["id"], "Ana")
    add_candidate(client, active["id"], "Luis")
    client.post(f"/elections/{active['id']}/activate", headers=ADMIN)

    only_active = client.get("/elections", params={"status": "active"}).json()
    assert [e["id"] for e in only_active] == [active["id"]]
    only_draft = client.get("/elections", params={"status": "draft"}).json()
    assert [e["id"] for e in only_draft] == [draft["id"]]
    assert client.get("/elections", params={"status": "otro"}).status_code == 422


def test_update_and_delete_draft(client):
    election = create_election(client)
    eid = election["id"]

    updated = client.patch(
        f"/elections/{eid}", json={"name": "Nombre actualizado"}, headers=ADMIN
    )
    assert updated.status_code == 200
    assert updated.json()["name"] == "Nombre actualizado"

    assert client.delete(f"/elections/{eid}", headers=ADMIN).status_code == 204
    assert client.get(f"/elections/{eid}").status_code == 404


def test_remove_candidate_and_get_candidate(client):
    election = create_election(client)
    eid = election["id"]
    candidate = add_candidate(client, eid, "Ana")

    fetched = client.get(f"/elections/{eid}/candidates/{candidate['id']}")
    assert fetched.status_code == 200
    assert fetched.json()["name"] == "Ana"

    assert client.delete(
        f"/elections/{eid}/candidates/{candidate['id']}", headers=ADMIN
    ).status_code == 204
    assert client.get(f"/elections/{eid}/candidates/{candidate['id']}").status_code == 404


def test_graphql_elections_with_nested_candidates(client):
    election = create_election(client, name="Elección GraphQL")
    add_candidate(client, election["id"], "Ana")
    add_candidate(client, election["id"], "Luis")

    query = """
    {
      elections {
        id name status isOpen candidateCount
        candidates { number name }
      }
    }
    """
    response = client.post("/graphql", json={"query": query})
    assert response.status_code == 200
    body = response.json()
    assert "errors" not in body
    data = body["data"]["elections"]
    assert len(data) == 1
    assert data[0]["name"] == "Elección GraphQL"
    assert data[0]["candidateCount"] == 2
    assert [c["name"] for c in data[0]["candidates"]] == ["Ana", "Luis"]


def test_graphql_single_election_and_missing(client):
    election = create_election(client)
    found = client.post(
        "/graphql",
        json={"query": "query($id: Int!) { election(id: $id) { id name } }", "variables": {"id": election["id"]}},
    ).json()
    assert found["data"]["election"]["id"] == election["id"]

    missing = client.post(
        "/graphql",
        json={"query": "{ election(id: 999) { id } }"},
    ).json()
    assert missing["data"]["election"] is None
