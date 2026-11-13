/**
 * Web App для листа «Облигации».
 *
 * Колонки:
 *   B — ISIN
 *   C — Рейтинг
 *   F — Выплат в год (купонов в год)
 *   I — Комментарий
 *
 * Деплой: Развернуть → Новое развёртывание → Веб-приложение
 *   Выполнять от имени: меня
 *   Доступ: все (или «все, у кого есть ссылка»)
 */

var SHEET_NAME = 'Income';
var COL = {
  ISIN: 2,       // B
  RATING: 3,     // C
  COUPONS: 6,    // F — «Выплат в год»
  COMMENT: 9,    // I
};

function doGet(e) {
  try {
    var rows = getIsinRows_();
    return json_({ ok: true, count: rows.length, data: rows });
  } catch (err) {
    return json_({ ok: false, error: String(err) }, 500);
  }
}

/**
 * Тело запроса (JSON):
 * {
 *   "isin": "RU000A107UU5",
 *   "rating": 231,              // опционально
 *   "couponsPerYear": 12,       // опционально — колонка «Выплат в год»
 *   "comment": "Флоатер"        // опционально
 * }
 *
 * Можно передать массив updates:
 * { "updates": [ { "isin": "...", "rating": 1, ... }, ... ] }
 */
function doPost(e) {
  try {
    var body = parseBody_(e);
    var updates = body.updates || [body];
    var results = [];

    for (var i = 0; i < updates.length; i++) {
      results.push(updateByIsin_(updates[i]));
    }

    return json_({ ok: true, results: results });
  } catch (err) {
    return json_({ ok: false, error: String(err) }, 500);
  }
}

/** ——— helpers ——— */

function getSheet_() {
  var ss = SpreadsheetApp.getActiveSpreadsheet();
  var sheet = ss.getSheetByName(SHEET_NAME);
  if (!sheet) {
    throw new Error('Лист «' + SHEET_NAME + '» не найден');
  }
  return sheet;
}

/** [{ row: 2, isin: "RU000A107UU5" }, ...] */
function getIsinRows_() {
  var sheet = getSheet_();
  var lastRow = sheet.getLastRow();
  if (lastRow < 2) {
    return [];
  }

  var values = sheet.getRange(2, COL.ISIN, lastRow, COL.ISIN).getValues();
  var out = [];

  for (var i = 0; i < values.length; i++) {
    var isin = String(values[i][0] || '').trim().toUpperCase();
    if (!isin) {
      continue;
    }
    out.push({ row: i + 2, isin: isin });
  }
  return out;
}

function findRowByIsin_(isin) {
  var needle = String(isin || '').trim().toUpperCase();
  if (!needle) {
    throw new Error('isin обязателен');
  }

  var rows = getIsinRows_();
  for (var i = 0; i < rows.length; i++) {
    if (rows[i].isin === needle) {
      return rows[i].row;
    }
  }
  throw new Error('ISIN не найден: ' + needle);
}

function updateByIsin_(item) {
  var isin = item.isin;
  var row = findRowByIsin_(isin);
  var sheet = getSheet_();
  var updated = { isin: String(isin).trim().toUpperCase(), row: row, fields: [] };

  if (Object.prototype.hasOwnProperty.call(item, 'rating')) {
    sheet.getRange(row, COL.RATING).setValue(item.rating);
    updated.fields.push('rating');
  }
  if (Object.prototype.hasOwnProperty.call(item, 'couponsPerYear')) {
    sheet.getRange(row, COL.COUPONS).setValue(item.couponsPerYear);
    updated.fields.push('couponsPerYear');
  }
  if (Object.prototype.hasOwnProperty.call(item, 'comment')) {
    var existing = String(sheet.getRange(row, COL.COMMENT).getValue() || '').trim();
    if (!existing) {
      sheet.getRange(row, COL.COMMENT).setValue(item.comment);
      updated.fields.push('comment');
    } else {
      updated.fields.push('comment_skipped');
    }
  }

  if (updated.fields.length === 0) {
    throw new Error('Нет полей для обновления (rating, couponsPerYear, comment)');
  }

  SpreadsheetApp.flush();
  return updated;
}

function parseBody_(e) {
  if (!e || !e.postData || !e.postData.contents) {
    throw new Error('Пустое тело запроса');
  }
  return JSON.parse(e.postData.contents);
}

function json_(obj) {
  return ContentService
    .createTextOutput(JSON.stringify(obj))
    .setMimeType(ContentService.MimeType.JSON);
}
