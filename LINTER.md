# Линтер

Проект использует `golangci-lint` для проверки качества кода.

## Установка

```bash
# macOS
brew install golangci-lint

# Linux
curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b $(go env GOPATH)/bin
```

## Использование

```bash
# Проверка кода
make lint

# Автоматическое исправление некоторых проблем
make lint-fix

# Или напрямую
golangci-lint run
golangci-lint run --fix
```

## Конфигурация

Конфигурация находится в файле `.golangci.yml`

### Включенные линтеры:
- `errcheck` - проверка обработки ошибок
- `govet` - стандартный анализатор Go
- `ineffassign` - обнаружение неэффективных присваиваний
- `staticcheck` - статический анализ
- `unused` - обнаружение неиспользуемого кода
- `misspell` - проверка орфографии
- `unconvert` - обнаружение ненужных конвертаций типов
- `unparam` - обнаружение неиспользуемых параметров функций
- `gocritic` - множество проверок производительности и стиля
- `gosec` - проверка безопасности
- `bodyclose` - проверка закрытия HTTP body
- `noctx` - проверка использования context
- `sqlclosecheck` - проверка закрытия SQL соединений
- `rowserrcheck` - проверка ошибок при работе с SQL rows
- `gocyclo` - проверка цикломатической сложности
- `dupl` - обнаружение дублирования кода
- `prealloc` - проверка предварительного выделения памяти для слайсов

## CI/CD

Линтер можно добавить в CI/CD pipeline:

```yaml
- name: Run linter
  run: make lint
```














