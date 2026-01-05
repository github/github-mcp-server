// Простая логика для дашборда: графики, транзакции и модал
const formatter = new Intl.NumberFormat('ru-RU', { style: 'currency', currency: 'RUB', maximumFractionDigits: 0 });
let transactions = [
  {id:1,type:'income',category:'salary',amount:85000,description:'Зарплата'},
  {id:2,type:'expense',category:'food',amount:8200,description:'Покупки в супермаркете'},
  {id:3,type:'expense',category:'transport',amount:1200,description:'Такси'},
];

function $(id){return document.getElementById(id)}

function updateBalances(){
  const totalIncome = transactions.filter(t=>t.type==='income').reduce((s,t)=>s+t.amount,0);
  const totalExpense = transactions.filter(t=>t.type==='expense').reduce((s,t)=>s+t.amount,0);
  const totalSavings = Math.max(0, totalIncome - totalExpense);
  const totalBalance = totalIncome - totalExpense;
  $('totalIncome').textContent = formatter.format(totalIncome);
  $('totalExpense').textContent = formatter.format(totalExpense);
  $('totalSavings').textContent = formatter.format(totalSavings);
  $('totalBalance').textContent = formatter.format(totalBalance);
}

// Charts
let financeChart = null;
let expenseChart = null;
function renderCharts(){
  const ctx = $('financeChart').getContext('2d');
  const months = ['Янв','Фев','Мар','Апр','Май','Июн','Июл','Авг','Сен','Окт','Ноя','Дек'];
  const incomeData = Array.from({length:12},(_,i)=> Math.round(50000 + Math.random()*50000));
  const expenseData = Array.from({length:12},(_,i)=> Math.round(15000 + Math.random()*30000));
  if(financeChart) financeChart.destroy();
  financeChart = new Chart(ctx,{
    type:'line',
    data:{labels:months,datasets:[{label:'Доходы',data:incomeData,borderColor:'#16a34a',backgroundColor:'rgba(16,163,127,0.08)',tension:0.3},{label:'Расходы',data:expenseData,borderColor:'#ef4444',backgroundColor:'rgba(239,68,68,0.06)',tension:0.3}]},
    options:{responsive:true,plugins:{legend:{position:'bottom'}}}
  });

  const ctx2 = $('expenseChart').getContext('2d');
  const categories = ['Продукты','Транспорт','Развлечения','Здоровье','Другое'];
  const catValues = [15000,3500,8200,5600,4000];
  if(expenseChart) expenseChart.destroy();
  expenseChart = new Chart(ctx2,{type:'doughnut',data:{labels:categories,datasets:[{data:catValues,backgroundColor:['#60a5fa','#34d399','#f97316','#f472b6','#a78bfa']}]},options:{responsive:true,plugins:{legend:{position:'bottom'}}}});
}

// Transactions
function renderTransactions(){
  const list = $('transactionsList');
  list.innerHTML = '';
  transactions.slice().reverse().forEach(t=>{
    const el = document.createElement('div');
    el.className = 'transaction';

    const meta = document.createElement('div');
    meta.className = 'meta';

    const category = document.createElement('div');
    category.className = 'category';
    category.textContent = (t.type==='income'? '📈 ' : '📉 ') + t.category;

    const desc = document.createElement('div');
    desc.className = 'desc';
    desc.textContent = t.description || '';

    meta.appendChild(category);
    meta.appendChild(desc);

    const amountEl = document.createElement('div');
    amountEl.className = 'amount';
    amountEl.textContent = (t.type==='income'? '+ ' : '- ') + formatter.format(t.amount);

    el.appendChild(meta);
    el.appendChild(amountEl);
    list.appendChild(el);
  });
}

function openModal(){
  $('transactionModal').style.display = 'flex';
}
function closeModal(){
  $('transactionModal').style.display = 'none';
}

function addTransaction(){
  openModal();
}

// Логика для модального окна входа
function openLoginModal() {
    document.getElementById('loginModal').style.display = 'flex';
}

function closeLoginModal() {
    document.getElementById('loginModal').style.display = 'none';
}

// Логика для модального окна регистрации
function openRegisterModal() {
    document.getElementById('registerModal').style.display = 'flex';
}

function closeRegisterModal() {
    document.getElementById('registerModal').style.display = 'none';
}

// Form handling
document.addEventListener('DOMContentLoaded',()=>{
  renderCharts();
  renderTransactions();
  updateBalances();
  const form = $('transactionForm');
  form.addEventListener('submit',(e)=>{
    e.preventDefault();
    const type = $('transactionType').value;
    const category = $('transactionCategory').value;
    const amount = Math.abs(Number($('transactionAmount').value)||0);
    const description = $('transactionDescription').value;
    if(amount<=0) return alert('Введите сумму больше 0');
    const id = Date.now();
    transactions.push({id,type,category,amount,description});
    renderTransactions();
    updateBalances();
    renderCharts();
    closeModal();
    form.reset();
  });

  // close modal when clicking outside content
  $('transactionModal').addEventListener('click',(e)=>{ if(e.target.id==='transactionModal') closeModal(); });

  // Добавляем обработчики для кнопок "Регистрация" и "Войти"
  const registerButton = document.querySelector('.btn-primary:nth-of-type(1)');
  const loginButton = document.querySelector('.btn-primary:nth-of-type(2)');

  registerButton.addEventListener('click', openRegisterModal);

  loginButton.addEventListener('click', openLoginModal);

  const loginForm = document.getElementById('loginForm');
  loginForm.addEventListener('submit', (e) => {
      e.preventDefault();
      const username = document.getElementById('loginUsername').value;
      const password = document.getElementById('loginPassword').value;
      // Authentication is disabled in this demo snapshot.
      // Do NOT ship hard-coded credentials in production. Masking demo behavior here.
      console.warn('Login attempt for user:', username);
      alert('В демо-режиме вход отключён.');
      closeLoginModal();
  });

  const registerForm = document.getElementById('registerForm');
  registerForm.addEventListener('submit', (e) => {
      e.preventDefault();
      const username = document.getElementById('registerUsername').value;
      const password = document.getElementById('registerPassword').value;
      const confirmPassword = document.getElementById('confirmPassword').value;

      if (password !== confirmPassword) {
          alert('Пароли не совпадают.');
          return;
      }

      alert('Регистрация успешна! Добро пожаловать, ' + username + '!');
      closeRegisterModal();
      registerForm.reset();
  });

  // Закрытие модального окна при клике вне контента
  document.getElementById('loginModal').addEventListener('click', (e) => {
      if (e.target.id === 'loginModal') closeLoginModal();
  });

  document.getElementById('registerModal').addEventListener('click', (e) => {
      if (e.target.id === 'registerModal') closeRegisterModal();
  });
});
