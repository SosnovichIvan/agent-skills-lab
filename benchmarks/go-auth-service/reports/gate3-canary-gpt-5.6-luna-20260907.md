# GATE-3 model canary

Дата: 2026-09-07

Профиль: `gpt-5.6-luna`, reasoning `medium`

Run ID: `gate3-canary-rerun-gpt-5.6-luna-20260907`

Canary запущен в новом пустом проекте на первых восьми задачах каталога с
`review_interval=4` и детерминированным architecture blocker после `1.4`.
Предыдущий run `gate3-canary-gpt-5.6-luna-20260907` был диагностическим: он
обнаружил потерю решения `reset` после recovery. После исправления controller
canary был полностью повторён с нуля.

## Результат

| Показатель | Значение |
| --- | ---: |
| Execution status | `complete` |
| Quality outcome | `pass` |
| Обычные задачи | 8/8 |
| Попытки обычных задач | 8 |
| Model architecture reviews | 2, оба `passed` |
| Forced recovery chunks | 1, `complete` |
| Фактические reset handoff | 1 |
| Handoff mismatches | 0 |
| Final state | `complete`, revision 21 |
| Wall time модели | 1087.003 s |
| Input tokens | 4,183,674 |
| Cached input tokens | 3,510,528 |
| Uncached input tokens | 673,146 |
| Output tokens | 66,496 |
| Reasoning output tokens | 16,978 |
| Total input + output | 4,250,170 |

`total` является техническим агрегатом input + output и не интерпретируется как
billed tokens. Cached и uncached input приведены отдельно.

Оба reviewer summary были сформированы на английском. На reset между
`foundation` и `registration-http` session изменился с
`01a07c05-9c5c-7732-beac-d0887abec9b2` на
`01a07c08-73c9-7fb1-b3c6-7a07be42502b`.

SHA-256 локального агрегата `metrics.json`:
`b67aa270d1ebb819b7b53983d23a56146d63e35909535e4a339001c8232df4f4`.
Raw model transcripts, generated project и worker packets не публикуются и
остаются в ignored benchmark output.
