import { BILLING_PRICING_VARS } from '../constants/billing.constants.js';
import {
  splitBillingExprAndRequestRules,
  tryParseRequestRuleExpr,
  unwrapOuterParens,
} from '../pages/Setting/Ratio/components/requestRuleExpr.js';

export const pricingType = (model) =>
  model.billing_mode === 'tiered_expr' ? 2 : model.quota_type;
const number = '(?:\\d+\\.?\\d*|\\.\\d+)(?:[eE][+-]?\\d+)?';
const term = new RegExp(
  `^(${BILLING_PRICING_VARS.map((v) => v.key).join('|')})\\s*\\*\\s*(${number})`,
);
const condition = new RegExp(
  `^(?:p|c|len)\\s*(?:<=|>=|<|>)\\s*${number}(?:\\s*&&\\s*(?:p|c|len)\\s*(?:<=|>=|<|>)\\s*${number})*$`,
);

// Only expose prices when the entire expression is understood. Never evaluate it.
function parseNode(value, conditions = [], depth = 0) {
  if (depth > 32) throw new Error('Expression too deep');
  const body = unwrapOuterParens(value.trim());
  let level = 0,
    quote = false,
    question = -1,
    nested = 0;
  for (let i = 0; i < body.length; i++) {
    const ch = body[i];
    if (ch === '"' && body[i - 1] !== '\\') quote = !quote;
    if (quote) continue;
    if (ch === '(') level++;
    if (ch === ')') level--;
    if (level !== 0) continue;
    if (ch === '?') {
      if (question < 0) question = i;
      else nested++;
    }
    if (ch === ':' && question >= 0) {
      if (nested) {
        nested--;
        continue;
      }
      const test = unwrapOuterParens(body.slice(0, question).trim());
      if (!condition.test(test)) throw new Error('Unsupported condition');
      return [
        ...parseNode(
          body.slice(question + 1, i),
          [...conditions, test],
          depth + 1,
        ),
        ...parseNode(
          body.slice(i + 1),
          [...conditions, `非（${test}）`],
          depth + 1,
        ),
      ];
    }
  }
  const match = body.match(/^tier\("([^"\\]*)",\s*([^()]*)\)$/);
  if (!match) throw new Error('Unsupported tier');
  let rest = match[2].trim();
  const prices = {};
  while (rest) {
    const item = rest.match(term);
    if (!item || !Number.isFinite(Number(item[2])))
      throw new Error('Unsupported price');
    const field = BILLING_PRICING_VARS.find((v) => v.key === item[1]).field;
    prices[field] = (prices[field] || 0) + Number(item[2]);
    rest = rest.slice(item[0].length).trim();
    if (!rest) break;
    if (!rest.startsWith('+') || !rest.slice(1).trim())
      throw new Error('Unsupported operator');
    rest = rest.slice(1).trim();
  }
  if (!Object.keys(prices).length) throw new Error('Empty tier');
  return [{ label: match[1], conditions, ...prices }];
}

export function parseDynamicPricing(expression) {
  try {
    const { billingExpr, requestRuleExpr } = splitBillingExprAndRequestRules(
      (expression || '').replace(/^v\d+:/, ''),
    );
    const rules = tryParseRequestRuleExpr(requestRuleExpr || '');
    if (!rules) throw new Error('Unsupported rule');
    return { tiers: parseNode(billingExpr), rules, supported: true };
  } catch {
    return { tiers: [], rules: [], supported: false };
  }
}

export function dynamicPriceData(
  expression,
  groupRatio,
  tokenUnit,
  displayPrice,
) {
  const parsed = parseDynamicPricing(expression);
  const unitLabel = tokenUnit === 'K' ? 'K' : 'M';
  const format = (value) =>
    displayPrice((value * groupRatio) / (unitLabel === 'K' ? 1000 : 1), 6);
  const items = parsed.supported
    ? BILLING_PRICING_VARS.filter((v) =>
        parsed.tiers.some((t) => t[v.field] !== undefined),
      ).map((v) => {
        const values = parsed.tiers.map((t) => t[v.field] ?? 0);
        const min = Math.min(...values),
          max = Math.max(...values);
        return {
          key: v.field,
          label: v.shortLabel,
          value: format(min) === format(max) ? format(min) : `${format(min)} ~ ${format(max)}`,
          suffix: ` / 1${unitLabel} tokens`,
        };
      })
    : [];
  return { ...parsed, isDynamic: true, unitLabel, format, items };
}

