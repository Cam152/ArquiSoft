# lab5-security — Laboratorio 5 (Network Segmentation, equipo 1b)

Todo lo del Laboratorio 5 vive aquí. El único archivo que se toca fuera de esta carpeta es `docker-compose.yml`, en la raíz del repo.

```
lab5-security/
├── README.md                  este archivo
├── l5_1b.md                   ENTREGABLE (guía técnica); se exporta a l5_1b.pdf
├── l5_1b.pdf                  (Fase 6) PDF que se sube a MiCampus
├── docs/
│   ├── distribucion_lab5.md   distribución del trabajo y orden de las fases
│   └── [SwArch_2026ii] - Laboratory 5.pdf   enunciado
├── design/
│   └── flujos-permitidos.md   escenario, zonas, matriz de flujos permitidos y bloqueados (Fase 1)
├── images/
│   ├── diagrams/              vistas arquitectónicas (.svg y .png): l5_despliegue.*
│   └── screenshots/           capturas de las pruebas: <caso>_<descripcion>.png
├── evidence/
│   ├── before/                salidas de texto con el compose del Prototipo 2 (Fase 0)
│   └── after/                 salidas de texto con el compose segmentado (Fase 4)
└── scripts/
    └── verify-segmentation.sh (Fase 0, Miguel) matriz de alcanzabilidad entre contenedores
```

## Convenciones

- **Redes:** todas las nuevas terminan en `-net` (`agr-edge-net`, `agr-votes-data-net`, …). Están declaradas en `docker-compose.yml` con `name:` explícito.
- **Capturas:** nómbrenlas con el id del caso de prueba (`design/flujos-permitidos.md` §4), p. ej. `B1_vote-lo-no-alcanza-voters-db.png`.
- **Evidencia de texto:** `evidence/<before|after>/matriz.txt`, `evidence/after/B3_auth-db.txt`, etc.
- **Imágenes en el .md:** usen rutas relativas a esta carpeta (`images/diagrams/l5_despliegue.png`) para que el PDF las encuentre.

## Flujo de trabajo en git

1. Ramas personales desde `lab5-security`: `lab5/<nombre>`.
2. Las redes ya están migradas (Fase 2.0). Cada uno solo cambia su sección de `l5_1b.md` y, si hace falta, algo de **sus** servicios (p. ej. el healthcheck de auth).
3. PR hacia `lab5-security`. Sebastián revisa y fusiona.
