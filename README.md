# 2FA CLI Tool

CLI-утилита для генерации одноразовых кодов аутентификации (TOTP) по секретному ключу.

## Использование

```bash
# Через аргумент командной строки
./2fa JBSWY3DPEHPK3PXP

# С пробелами в ключе
./2fa "qvah jwnp ndeu oq4a xir4 nags awow vgxw"

# Интерактивный режим
./2fa
Введите секретный ключ (Base32): JBSWY3DPEHPK3PXP
```

## Алгоритм

Реализован стандарт [RFC 6238](https://tools.ietf.org/html/rfc6238) (TOTP — Time-based One-Time Password):

1. Секретный ключ (Base32) декодируется в байты
2. Текущее Unix-время делится на шаг 30 секунд
3. Временной шаг переводится в 8-байтное Big Endian представление
4. Вычисляется HMAC-SHA1 хэш от времени с использованием ключа
5. Динамическое усечение (Dynamic Truncation) извлекает 4 байта из хэша
6. Результат преобразуется в 6-значный код (с ведущими нулями)

Алгоритм идентичен тому, который используется в Google Authenticator, Authy, Microsoft Authenticator и [2fa.live](https://2fa.live/).

## Сборка

```bash
go build -o 2fa
```
