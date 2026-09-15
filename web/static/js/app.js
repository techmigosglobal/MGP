document.addEventListener('alpine:init', () => {
  Alpine.data('gatePassForm', () => ({
    passType: 'RETURNABLE',
    addItem() {
      const first = document.querySelector('#item-rows .item-row');
      if (!first) return;
      const row = first.cloneNode(true);
      row.querySelectorAll('input').forEach((input) => { input.value = input.name === 'unit[]' ? 'NOS' : input.name === 'quantity[]' ? '1' : ''; });
      row.querySelectorAll('select').forEach((select) => { select.selectedIndex = 0; });
      document.querySelector('#item-rows').appendChild(row);
    }
  }));
});

document.addEventListener('htmx:configRequest', (event) => {
  const token = document.querySelector('meta[name="csrf-token"]')?.content;
  if (token) event.detail.headers['X-CSRF-Token'] = token;
});
