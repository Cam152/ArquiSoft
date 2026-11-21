# Resultados R1 a R4 (generado por make_charts.py)

## R1 · Lectura sin proxy (línea base)

`http://localhost:8080` · resumen en `results/r1-base/`

| VUs | req/s | p50 | p95 | p99 | error |
|---:|---:|---:|---:|---:|---:|
| 50 | 97.3 | 13 ms | 44 ms | 65 ms | 0.00 % |
| 200 | 385.9 | 16 ms | 85 ms | 222 ms | 0.00 % |
| 500 | 844.7 | 61 ms | 272 ms | 511 ms | 0.00 % |
| 1000 | 1078.9 | 468 ms | 749 ms | 1.37 s | 0.00 % |
| 1500 | 1079.7 | 928 ms | 1.18 s | 2.68 s | 0.00 % |

| VUs | Endpoint | p50 | p95 | p99 | error |
|---:|---|---:|---:|---:|---:|
| 50 | `GET /elections` | 14 ms | 52 ms | 90 ms | 0.00 % |
| 50 | `GET /results/{id}` | 12 ms | 35 ms | 51 ms | 0.00 % |
| 200 | `GET /elections` | 17 ms | 117 ms | 340 ms | 0.00 % |
| 200 | `GET /results/{id}` | 16 ms | 63 ms | 96 ms | 0.00 % |
| 500 | `GET /elections` | 71 ms | 296 ms | 730 ms | 0.00 % |
| 500 | `GET /results/{id}` | 54 ms | 254 ms | 309 ms | 0.00 % |
| 1000 | `GET /elections` | 473 ms | 792 ms | 1.77 s | 0.00 % |
| 1000 | `GET /results/{id}` | 463 ms | 707 ms | 799 ms | 0.00 % |
| 1500 | `GET /elections` | 941 ms | 1.74 s | 3.04 s | 0.00 % |
| 1500 | `GET /results/{id}` | 915 ms | 1.13 s | 1.20 s | 0.00 % |

```
contenedor                         CPU prom % CPU máx % mem máx MiB
agr-election-lo                         225.6      535.9        394.9
agr-elections-db                         74.9      232.2        128.8
agr-broker-as                            24.0      223.1        224.2
arquisoft-agr-gateway-or-1               49.8      204.0        251.2
agr-audit-db                             10.8       73.4        417.5
agr-votes-db                             11.3       73.0        274.4
agr-audit-lo                              1.4       52.0        106.4
agr-auth-db                               1.2       12.4         60.1
agr-voters-db                             1.1       11.7         40.1
agr-results-lo                            3.7       10.5         22.2
agr-auth-lo                               0.7       10.0        250.1
agr-results-db                            2.8        9.5         14.4
agr-web-fr                                0.4        7.5         41.8
agr-voter-lo                              0.6        6.8        170.2
agr-vote-lo                               0.4        6.2         15.8
```

## R1 · Lectura con proxy (= R4, 1 réplica)

`http://localhost/api` · resumen en `results/r1-proxy/`

| VUs | req/s | p50 | p95 | p99 | error |
|---:|---:|---:|---:|---:|---:|
| 50 | 96.9 | 19 ms | 51 ms | 89 ms | 0.00 % |
| 200 | 371.8 | 37 ms | 137 ms | 232 ms | 0.00 % |
| 500 | 857.8 | 63 ms | 278 ms | 574 ms | 0.00 % |
| 1000 | 1054.9 | 474 ms | 781 ms | 1.32 s | 0.00 % |
| 1500 | 1035.9 | 987 ms | 1.36 s | 3.29 s | 0.00 % |

| VUs | Endpoint | p50 | p95 | p99 | error |
|---:|---|---:|---:|---:|---:|
| 50 | `GET /elections` | 23 ms | 59 ms | 107 ms | 0.00 % |
| 50 | `GET /results/{id}` | 16 ms | 40 ms | 58 ms | 0.00 % |
| 200 | `GET /elections` | 45 ms | 163 ms | 380 ms | 0.00 % |
| 200 | `GET /results/{id}` | 33 ms | 87 ms | 144 ms | 0.00 % |
| 500 | `GET /elections` | 71 ms | 309 ms | 717 ms | 0.00 % |
| 500 | `GET /results/{id}` | 57 ms | 247 ms | 338 ms | 0.00 % |
| 1000 | `GET /elections` | 488 ms | 863 ms | 1.75 s | 0.00 % |
| 1000 | `GET /results/{id}` | 460 ms | 714 ms | 849 ms | 0.00 % |
| 1500 | `GET /elections` | 981 ms | 2.03 s | 4.56 s | 0.00 % |
| 1500 | `GET /results/{id}` | 992 ms | 1.27 s | 1.39 s | 0.00 % |

```
contenedor                         CPU prom % CPU máx % mem máx MiB
agr-election-lo                         230.6      547.9        426.4
agr-elections-db                         79.0      257.7        136.7
agr-broker-as                            21.6      237.3        259.8
arquisoft-agr-gateway-or-1               51.2      204.0        264.5
agr-audit-db                             10.8       70.4        349.7
agr-votes-db                             10.9       68.3        280.6
agr-proxy-rp                              7.6       17.4         38.7
agr-results-db                            3.0       12.7         15.6
agr-auth-lo                               0.7       11.4        296.4
agr-web-fr                                0.5       11.1         44.3
agr-results-lo                            3.7       10.3         15.5
agr-voters-db                             1.0        9.2         41.3
agr-auth-db                               1.0        8.6         61.4
agr-voter-lo                              0.5        7.1        261.1
agr-vote-lo                               0.5        6.3         13.4
agr-audit-lo                              0.5        6.1         54.3
```

## R4 · Lectura con proxy y 3 réplicas del gateway

`http://localhost/api` · resumen en `results/r4-3x/`

| VUs | req/s | p50 | p95 | p99 | error |
|---:|---:|---:|---:|---:|---:|
| 50 | 96.9 | 19 ms | 54 ms | 101 ms | 0.00 % |
| 200 | 365.7 | 44 ms | 150 ms | 227 ms | 0.00 % |
| 500 | 810.5 | 113 ms | 327 ms | 543 ms | 0.00 % |
| 1000 | 1044.0 | 473 ms | 728 ms | 1.88 s | 0.00 % |
| 1500 | 1022.1 | 991 ms | 1.31 s | 4.09 s | 0.00 % |

| VUs | Endpoint | p50 | p95 | p99 | error |
|---:|---|---:|---:|---:|---:|
| 50 | `GET /elections` | 22 ms | 62 ms | 111 ms | 0.00 % |
| 50 | `GET /results/{id}` | 17 ms | 45 ms | 86 ms | 0.00 % |
| 200 | `GET /elections` | 54 ms | 171 ms | 275 ms | 0.00 % |
| 200 | `GET /results/{id}` | 38 ms | 119 ms | 152 ms | 0.00 % |
| 500 | `GET /elections` | 123 ms | 355 ms | 665 ms | 0.00 % |
| 500 | `GET /results/{id}` | 106 ms | 292 ms | 390 ms | 0.00 % |
| 1000 | `GET /elections` | 482 ms | 773 ms | 2.48 s | 0.00 % |
| 1000 | `GET /results/{id}` | 464 ms | 695 ms | 764 ms | 0.00 % |
| 1500 | `GET /elections` | 1.01 s | 1.73 s | 4.53 s | 0.00 % |
| 1500 | `GET /results/{id}` | 974 ms | 1.25 s | 1.34 s | 0.00 % |

```
contenedor                         CPU prom % CPU máx % mem máx MiB
agr-election-lo                         230.6      503.8        430.1
agr-broker-as                            16.1      228.7        276.1
agr-elections-db                         71.1      216.7        136.1
arquisoft-agr-gateway-or-2               25.0       94.8        142.9
arquisoft-agr-gateway-or-3               23.3       89.1        142.7
arquisoft-agr-gateway-or-1               23.7       83.0        170.5
agr-votes-db                             11.3       67.0        284.4
agr-audit-db                             11.0       65.9        352.6
agr-proxy-rp                              8.8       51.8         40.9
agr-results-lo                            4.2       16.6         11.2
agr-results-db                            3.1       16.4         17.7
agr-auth-db                               1.1       14.7         60.6
agr-voters-db                             1.1       14.3         40.8
agr-audit-lo                              0.5       13.7         55.1
agr-web-fr                                0.6       13.4         41.9
agr-vote-lo                               0.5        9.2         14.1
agr-voter-lo                              0.7        8.0        258.9
agr-auth-lo                               0.6        6.0        316.8
```

## R2 · Escritura sin proxy (línea base)

`http://localhost:8080` · resumen en `results/r2-base/`

| VUs | req/s | p50 | p95 | p99 | error |
|---:|---:|---:|---:|---:|---:|
| 10 | 18.0 | 54 ms | 112 ms | 167 ms | 0.00 % |
| 25 | 45.0 | 60 ms | 152 ms | 190 ms | 0.00 % |
| 50 | 90.0 | 68 ms | 156 ms | 344 ms | 0.00 % |
| 100 | 180.7 | 55 ms | 154 ms | 455 ms | 0.00 % |
| 200 | 224.1 | 69 ms | 1.02 s | 1.69 s | 0.00 % |

| VUs | Endpoint | p50 | p95 | p99 | error |
|---:|---|---:|---:|---:|---:|
| 10 | `POST /auth/login` | 86 ms | 122 ms | 173 ms | 0.00 % |
| 10 | `POST /votes` | 20 ms | 33 ms | 39 ms | 0.00 % |
| 25 | `POST /auth/login` | 86 ms | 170 ms | 213 ms | 0.00 % |
| 25 | `POST /votes` | 17 ms | 33 ms | 41 ms | 0.00 % |
| 50 | `POST /auth/login` | 89 ms | 175 ms | 508 ms | 0.00 % |
| 50 | `POST /votes` | 17 ms | 33 ms | 56 ms | 0.00 % |
| 100 | `POST /auth/login` | 83 ms | 238 ms | 665 ms | 0.00 % |
| 100 | `POST /votes` | 14 ms | 25 ms | 32 ms | 0.00 % |
| 200 | `POST /auth/login` | 793 ms | 1.44 s | 1.84 s | 0.00 % |
| 200 | `POST /votes` | 18 ms | 30 ms | 43 ms | 0.00 % |

```
contenedor                         CPU prom % CPU máx % mem máx MiB
agr-auth-lo                             330.3      928.0        310.4
agr-broker-as                            28.3      213.6        224.5
agr-voter-lo                              9.6       83.6        257.0
arquisoft-agr-gateway-or-1               16.6       78.9        144.5
agr-election-lo                          19.8       75.6        378.1
agr-votes-db                             12.9       67.6        264.2
agr-audit-db                             11.4       66.7        415.3
agr-audit-lo                              3.7       42.7        113.4
agr-elections-db                          2.8       18.0         90.9
agr-voters-db                             2.0       14.4         39.9
agr-results-lo                            1.6       13.0         17.2
agr-auth-db                               2.4       13.0         61.2
agr-vote-lo                               3.2       10.6         17.6
agr-web-fr                                0.5        8.7         38.9
agr-results-db                            2.1        8.2         16.9
```

## R2 · Escritura con proxy

`http://localhost/api` · resumen en `results/r2-proxy/`

| VUs | req/s | p50 | p95 | p99 | error |
|---:|---:|---:|---:|---:|---:|
| 10 | 18.7 | 50 ms | 109 ms | 122 ms | 0.00 % |
| 25 | 45.3 | 59 ms | 115 ms | 192 ms | 0.00 % |
| 50 | 90.0 | 68 ms | 154 ms | 273 ms | 0.00 % |
| 100 | 175.9 | 67 ms | 204 ms | 549 ms | 0.00 % |
| 200 | 227.1 | 72 ms | 926 ms | 1.56 s | 0.00 % |

| VUs | Endpoint | p50 | p95 | p99 | error |
|---:|---|---:|---:|---:|---:|
| 10 | `POST /auth/login` | 82 ms | 117 ms | 150 ms | 0.00 % |
| 10 | `POST /votes` | 15 ms | 28 ms | 31 ms | 0.00 % |
| 25 | `POST /auth/login` | 90 ms | 122 ms | 228 ms | 0.00 % |
| 25 | `POST /votes` | 15 ms | 37 ms | 46 ms | 0.00 % |
| 50 | `POST /auth/login` | 91 ms | 179 ms | 382 ms | 0.00 % |
| 50 | `POST /votes` | 15 ms | 30 ms | 49 ms | 0.00 % |
| 100 | `POST /auth/login` | 101 ms | 283 ms | 787 ms | 0.00 % |
| 100 | `POST /votes` | 14 ms | 26 ms | 37 ms | 0.00 % |
| 200 | `POST /auth/login` | 755 ms | 1.35 s | 1.67 s | 0.00 % |
| 200 | `POST /votes` | 17 ms | 28 ms | 37 ms | 0.00 % |

```
contenedor                         CPU prom % CPU máx % mem máx MiB
agr-auth-lo                             316.8      926.2        314.4
agr-broker-as                            26.6      217.6        268.6
arquisoft-agr-gateway-or-1               17.3       77.0        144.0
agr-audit-db                             11.3       65.4        357.4
agr-votes-db                             12.4       64.1        286.0
agr-election-lo                          13.8       38.5        391.2
agr-vote-lo                               3.4       20.1         21.6
agr-voters-db                             2.1       18.1         41.9
agr-auth-db                               2.4       18.0         61.7
agr-audit-lo                              3.8       17.0         55.6
agr-voter-lo                              2.7       16.5        259.2
agr-proxy-rp                              1.7       14.8         16.1
agr-results-lo                            1.8       13.6         11.3
agr-elections-db                          2.7        9.7         90.5
agr-results-db                            2.2        8.3         15.6
agr-web-fr                                0.4        7.2         45.0
```

## R1 · Lectura con proxy, primera versión (sin keepalive hacia el gateway)

`http://localhost/api` · resumen en `results/r1-proxy-sin-keepalive/`

| VUs | req/s | p50 | p95 | p99 | error |
|---:|---:|---:|---:|---:|---:|
| 50 | 96.7 | 20 ms | 45 ms | 74 ms | 0.00 % |
| 200 | 368.1 | 44 ms | 155 ms | 196 ms | 0.00 % |
| 500 | 794.1 | 103 ms | 378 ms | 510 ms | 9.27 % |
| 1000 | 1352.3 | 82 ms | 974 ms | 1.41 s | 61.53 % |
| 1500 | 1209.6 | 618 ms | 1.70 s | 2.74 s | 46.98 % |

| VUs | Endpoint | p50 | p95 | p99 | error |
|---:|---|---:|---:|---:|---:|
| 50 | `GET /elections` | 21 ms | 45 ms | 63 ms | 0.00 % |
| 50 | `GET /results/{id}` | 19 ms | 45 ms | 77 ms | 0.00 % |
| 200 | `GET /elections` | 51 ms | 161 ms | 200 ms | 0.00 % |
| 200 | `GET /results/{id}` | 39 ms | 136 ms | 192 ms | 0.00 % |
| 500 | `GET /elections` | 106 ms | 360 ms | 485 ms | 9.25 % |
| 500 | `GET /results/{id}` | 100 ms | 415 ms | 513 ms | 9.29 % |
| 1000 | `GET /elections` | 84 ms | 1.04 s | 1.50 s | 60.53 % |
| 1000 | `GET /results/{id}` | 79 ms | 905 ms | 1.15 s | 62.54 % |
| 1500 | `GET /elections` | 673 ms | 1.79 s | 2.90 s | 43.76 % |
| 1500 | `GET /results/{id}` | 563 ms | 1.62 s | 2.48 s | 50.19 % |

```
contenedor                         CPU prom % CPU máx % mem máx MiB
agr-proxy-rp                            170.6     1114.3         45.6
agr-election-lo                         159.0      511.7        411.4
agr-elections-db                         47.9      244.1        150.5
agr-broker-as                            26.4      223.5        243.0
arquisoft-agr-gateway-or-1               40.6      189.3        247.1
agr-votes-db                             10.1       64.0        272.8
agr-audit-db                             10.1       63.5        417.6
agr-results-lo                            3.0       19.0         14.8
agr-auth-lo                               0.7       11.4        303.2
agr-results-db                            2.5       11.0         18.6
agr-vote-lo                               0.6       10.5         14.6
agr-audit-lo                              0.4        9.9        108.1
agr-voters-db                             1.1        9.7         41.0
agr-auth-db                               1.1        9.3         61.5
agr-voter-lo                              0.6        7.2        259.7
agr-web-fr                                0.5        6.7         42.8
```

## R2 · Escritura con proxy, primera versión (sin keepalive)

`http://localhost/api` · resumen en `results/r2-proxy-sin-keepalive/`

| VUs | req/s | p50 | p95 | p99 | error |
|---:|---:|---:|---:|---:|---:|
| 10 | 18.7 | 52 ms | 112 ms | 122 ms | 0.00 % |
| 25 | 45.6 | 61 ms | 117 ms | 162 ms | 0.00 % |
| 50 | 90.0 | 57 ms | 193 ms | 283 ms | 0.00 % |
| 100 | 173.3 | 69 ms | 207 ms | 484 ms | 0.00 % |
| 200 | 228.9 | 78 ms | 1.00 s | 1.56 s | 0.00 % |

| VUs | Endpoint | p50 | p95 | p99 | error |
|---:|---|---:|---:|---:|---:|
| 10 | `POST /auth/login` | 85 ms | 116 ms | 128 ms | 0.00 % |
| 10 | `POST /votes` | 17 ms | 28 ms | 34 ms | 0.00 % |
| 25 | `POST /auth/login` | 88 ms | 125 ms | 183 ms | 0.00 % |
| 25 | `POST /votes` | 15 ms | 28 ms | 45 ms | 0.00 % |
| 50 | `POST /auth/login` | 98 ms | 211 ms | 373 ms | 0.00 % |
| 50 | `POST /votes` | 16 ms | 30 ms | 39 ms | 0.00 % |
| 100 | `POST /auth/login` | 153 ms | 240 ms | 704 ms | 0.00 % |
| 100 | `POST /votes` | 17 ms | 29 ms | 41 ms | 0.00 % |
| 200 | `POST /auth/login` | 752 ms | 1.34 s | 1.67 s | 0.00 % |
| 200 | `POST /votes` | 17 ms | 28 ms | 36 ms | 0.00 % |

```
contenedor                         CPU prom % CPU máx % mem máx MiB
agr-auth-lo                             321.7      927.1        317.7
agr-broker-as                            21.3      218.3        252.6
arquisoft-agr-gateway-or-1               17.3       90.0        145.9
agr-votes-db                             13.2       68.4        280.5
agr-election-lo                          16.3       68.4        382.7
agr-audit-db                             11.9       64.4        414.3
agr-audit-lo                              3.5       13.5         54.9
agr-auth-db                               2.3       13.3         61.1
agr-voters-db                             1.9       13.2         39.2
agr-vote-lo                               3.4       12.5         19.7
agr-voter-lo                              2.4       11.3        257.7
agr-elections-db                          2.7        9.7         91.3
agr-results-db                            2.2        8.4         18.6
agr-proxy-rp                              2.4        8.2         18.7
agr-results-lo                            1.7        8.0         15.1
agr-web-fr                                0.5        7.1         42.5
```

## R4 · 3 réplicas, primera versión del proxy (sin keepalive)

`http://localhost/api` · resumen en `results/r4-3x-sin-keepalive/`

| VUs | req/s | p50 | p95 | p99 | error |
|---:|---:|---:|---:|---:|---:|
| 50 | 98.3 | 13 ms | 44 ms | 67 ms | 0.00 % |
| 200 | 372.5 | 30 ms | 135 ms | 219 ms | 0.00 % |
| 500 | 831.9 | 89 ms | 283 ms | 641 ms | 0.00 % |
| 1000 | 970.3 | 535 ms | 973 ms | 1.58 s | 0.00 % |
| 1500 | 981.8 | 1.01 s | 1.73 s | 4.39 s | 0.00 % |

| VUs | Endpoint | p50 | p95 | p99 | error |
|---:|---|---:|---:|---:|---:|
| 50 | `GET /elections` | 15 ms | 46 ms | 74 ms | 0.00 % |
| 50 | `GET /results/{id}` | 12 ms | 41 ms | 62 ms | 0.00 % |
| 200 | `GET /elections` | 33 ms | 149 ms | 287 ms | 0.00 % |
| 200 | `GET /results/{id}` | 28 ms | 109 ms | 171 ms | 0.00 % |
| 500 | `GET /elections` | 97 ms | 301 ms | 783 ms | 0.00 % |
| 500 | `GET /results/{id}` | 84 ms | 269 ms | 355 ms | 0.00 % |
| 1000 | `GET /elections` | 544 ms | 1.03 s | 1.55 s | 0.00 % |
| 1000 | `GET /results/{id}` | 526 ms | 922 ms | 2.01 s | 0.00 % |
| 1500 | `GET /elections` | 1.03 s | 1.80 s | 4.22 s | 0.00 % |
| 1500 | `GET /results/{id}` | 1.00 s | 1.65 s | 4.70 s | 0.00 % |

```
contenedor                         CPU prom % CPU máx % mem máx MiB
agr-election-lo                         216.6      509.0        420.4
agr-proxy-rp                             27.8      373.4         52.3
agr-broker-as                            25.8      220.2        261.5
agr-elections-db                         64.2      207.5        128.1
arquisoft-agr-gateway-or-1               25.5      130.8        160.1
arquisoft-agr-gateway-or-2               25.6       99.0        140.0
arquisoft-agr-gateway-or-3               25.4       96.0        146.2
agr-votes-db                             11.6       67.9        278.6
agr-audit-db                             11.5       66.8        421.7
agr-auth-db                               1.3       16.0         58.6
agr-voters-db                             1.2       15.8         38.6
agr-results-db                            3.0       14.6         17.7
agr-web-fr                                0.6       12.8         39.9
agr-results-lo                            4.2       11.6         14.9
agr-voter-lo                              0.6        9.9        258.1
agr-audit-lo                              0.5        7.4         56.1
agr-auth-lo                               0.6        7.2        317.1
agr-vote-lo                               0.5        6.9         14.1
```
