# Correção dos totais de energia OBIS (4 tarifas + registos totais 20/21)

- **Data:** 2026-07-06
- **Componentes:** `internal/parser/obis.go`, `internal/csv/writer.go`, testes do parser
- **Contador de referência:** Landis+Gyr `LGZ4\2ZMD4054407.B31` (notação OBIS curta/plana)

## Problema

O parser só lê duas tarifas por direção de energia e calcula os totais como soma dessas duas:

- `EnergyImportTotal = 8.1 + 8.2` ([obis.go:252-255](../../../internal/parser/obis.go#L252-L255))
- `EnergyExportTotal = 9.1 + 9.2` ([obis.go:257-260](../../../internal/parser/obis.go#L257-L260))

O contador está configurado com **4 tarifas** e disponibiliza registos totais oficiais que o parser ignora:

- `20` = energia total acumulada +A (importada), independente de tarifas
- `21` = energia total acumulada -A (exportada), independente de tarifas

Consequência: os totais exportados no CSV subcontam a energia. Confirmado com um dump RAW real de 2026-07-06:

**Exportação (-A):**

| Registo | Valor |
|---|---|
| `9.1` | 1 507 421 |
| `9.2` | 1 022 150 |
| `9.3` | 2 917 125 |
| `9.4` | 1 027 411 |
| soma 9.1..9.4 | 6 474 107 |
| `21` (total oficial) | **6 474 108** |
| `9.1 + 9.2` (valor atual do software) | 2 529 571 |

**Importação (+A):**

| Registo | Valor |
|---|---|
| `8.1` | 50 714 |
| `8.2` | 34 362 |
| `8.3` | 72 590 |
| `8.4` | 34 175 |
| soma 8.1..8.4 | 191 841 |
| `20` (total oficial) | **191 841** (coincidência exata) |
| `8.1 + 8.2` (valor atual do software) | 85 076 |

Como a distribuição de energia pelas 4 tarifas varia ao longo dos meses, somar apenas T1+T2 produz um fator de subcontagem que também varia mês a mês — o que explica as diferenças mensais observadas face à E-Redes.

## Fora de âmbito

- **Cálculo de incrementos / "picos de 500 kWh":** este repositório apenas captura leituras *acumuladas* e escreve-as no CSV. Não existe aqui nenhuma lógica de diferenças entre leituras (confirmado por busca). Esse cálculo é feito por um processo a jusante (reconciliação E-Redes / dashboard). Esta alteração corrige os *dados de entrada* desse processo, mas não os picos em si.
- Totais de energia reativa (registos `22`, `23`, `24`, `25`) e refactorings não relacionados.

## Formato dos tokens (confirmado no RAW)

O contador usa a notação curta e plana, sem prefixo `1-0:` e sem códigos IEC completos:

```
8.3(00072590*kWh)
9.3(02917125*kWh)
20(00191841*kWh)
21(06474108*kWh)
```

Cada registo atual é seguido de snapshots históricos de faturação (`8.3*80(...)`, `21*79(...)`, …). O parser já resolve isto corretamente: `scanObservedValues` guarda a **primeira ocorrência** de cada código normalizado ([obis.go:117](../../../internal/parser/obis.go#L117)), e o valor atual aparece sempre antes dos históricos `*NN`. `normalizeOBISCode` reduz `8.3*80` a `8.3`, pelo que os históricos colidem com o código atual mas são ignorados por já existir entrada.

Os aliases incluem na mesma o código IEC completo por robustez para outros contadores.

## Design

### 1. Parser — novos campos e leitura (`internal/parser/obis.go`)

Adicionar 4 campos de tarifa a `MeterReading`:

```go
EnergyImportT3 *float64 // 8.3
EnergyImportT4 *float64 // 8.4
EnergyExportT3 *float64 // 9.3
EnergyExportT4 *float64 // 9.4
```

Ler os 4 tarifários e os dois totais oficiais em `Parse()`:

```go
reading.EnergyImportT3 = extractFloat(observedValues, "1.8.3", "8.3")
reading.EnergyImportT4 = extractFloat(observedValues, "1.8.4", "8.4")
reading.EnergyExportT3 = extractFloat(observedValues, "2.8.3", "9.3")
reading.EnergyExportT4 = extractFloat(observedValues, "2.8.4", "9.4")

importGrandTotal := extractFloat(observedValues, "1.8.0", "20") // OBIS 20 = total +A
exportGrandTotal := extractFloat(observedValues, "2.8.0", "21") // OBIS 21 = total -A
```

Os totais oficiais são **variáveis locais**, não campos da struct, para manter a correspondência 1:1 entre campos da struct e colunas do CSV (não há coluna dedicada de grand-total — decisão do utilizador).

### 2. Cálculo dos totais — preferir o registo oficial, com fallback

`calculateTotals` passa a receber os dois totais oficiais como argumentos. Lógica para cada direção:

```
Total = registo oficial (OBIS 20 / OBIS 21)      se presente
      senão soma das tarifas presentes            se T1 e T2 presentes
            (T1 + T2 [+ T3 se presente] [+ T4 se presente])
      senão nil
```

Racional:
- Usa o valor autoritário do contador quando existe (o caso deste contador).
- É imune a uma tarifa em falta num dump (que, na soma, criaria uma queda/pico artificial).
- É à prova de futuro se o número de tarifas mudar.
- O ramo de fallback preserva exatamente o comportamento legado (exige T1 e T2), pelo que os contadores de 2 tarifas sem registo total continuam a funcionar como antes.

### 3. CSV — 4 colunas novas no fim (`internal/csv/writer.go`)

Acrescentar ao fim de `csvHeaders` e de `readingToRow`, a seguir a `reactive_quadrant_49_2_kvarh`:

```
energy_import_t3_kwh
energy_import_t4_kwh
energy_export_t3_kwh
energy_export_t4_kwh
```

Colunas no **fim** para não deslocar posições existentes. As colunas `energy_import_total_kwh` e `energy_export_total_kwh` mantêm nome e posição, mas passam a transportar OBIS 20 / OBIS 21 respetivamente. Atualizar também `ParsedFieldCount` para contar os 4 campos novos.

### 4. Testes (`internal/parser/obis_test.go`)

- **Fixture com o RAW real** de 2026-07-06. Asserções:
  - `EnergyImportT1..T4` = 50714 / 34362 / 72590 / 34175
  - `EnergyExportT1..T4` = 1507421 / 1022150 / 2917125 / 1027411
  - `EnergyImportTotal == 191841` (OBIS 20)
  - `EnergyExportTotal == 6474108` (OBIS 21) — **não** 6474107; prova que o total vem do registo oficial e não da soma das tarifas.
- **Teste sintético** com grand-total ≠ soma, a confirmar que o grand-total é preferido.
- **Teste de fallback:** dump com T1..T4 mas sem registo total → total = soma das 4 tarifas.
- Confirmar que `TestParseStandardOBISFrame` (só T1/T2, sem total → fallback = soma) e `TestParseLegacyShortCodes` continuam verdes.

## Nota operacional (fora deste repositório)

No deploy, `energy_export_total_kwh` salta de ~2 529 571 para ~6 474 108 e `energy_import_total_kwh` de ~85 076 para ~191 841. Um processo a jusante que calcule incrementos verá um degrau artificial único nesse instante. Recomendação: registar a data/hora do deploy para que a reconciliação trate o degrau (ignorar o intervalo que atravessa a atualização, ou fazer re-baseline). Aceite pelo utilizador como salto único.

## Critérios de aceitação

1. Parser lê `8.3`, `8.4`, `9.3`, `9.4`, `20`, `21` a partir do RAW real.
2. `EnergyExportTotal == 6474108` e `EnergyImportTotal == 191841` para o RAW real.
3. CSV inclui as 4 colunas novas no fim; colunas existentes inalteradas em nome e posição.
4. Testes existentes continuam a passar; novos testes verdes.
5. `go build ./...` e `go test ./...` sem erros.
