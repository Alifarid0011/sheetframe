// Source data and requests for the same Go engine used by manual controls.
export const workflows = {
  sales: {
    title: 'Studio orders', file: 'studio_orders.csv', source: 'sample.csv',
    label: 'Sales totals by region', question: 'Which region brings in the most revenue?',
    steps: 'Sum revenue by region, then sort the totals from highest to lowest.',
    request: { action: 'transform', group: { keys: ['region'], aggregations: [{ column: 'revenue', op: 'sum', as: 'sum_revenue' }] }, sort: { column: 'sum_revenue', descending: true } },
  },
  expenses: {
    title: 'Team expenses', file: 'team_expenses.csv', source: 'expenses.csv',
    label: 'Expenses by category', question: 'Where is the team spending the most?',
    steps: 'Sum amount by category, then sort the totals from highest to lowest.',
    request: { action: 'transform', group: { keys: ['category'], aggregations: [{ column: 'amount', op: 'sum', as: 'sum_amount' }] }, sort: { column: 'sum_amount', descending: true } },
  },
  cleanup: {
    title: 'Invoice cleanup', file: 'invoices.csv', source: 'invoices.csv',
    label: 'Clean up missing amounts', question: 'Which invoices have an amount ready to use?',
    steps: 'Keep rows whose amount is not empty. Zero is a valid amount and stays in the result.',
    request: { action: 'transform', filter: { column: 'amount', op: 'not_null' } },
  },
};
