[file name]: client/website/js/app.js
[file content begin]
document.addEventListener('DOMContentLoaded', async function() {
    const newsContainer = document.getElementById('news-container');
    const recentContainer = document.getElementById('recent-container');
    const filterButtons = document.querySelectorAll('.filter-btn');
    const searchInput = document.querySelector('.search-input');
    const searchButton = document.querySelector('.search-btn');
    const searchTags = document.querySelectorAll('.search-tag');
    let currentFilter = 'all';
    let newsData = [];

    async function init() {
        try {
            // Показываем загрузку
            showLoading();
            
            // Проверяем доступность API
            if (!await checkAPIHealth()) {
                showWarning();
            }
            
            // Загружаем данные через API
            const [popularPosts, recentPosts] = await Promise.allSettled([
                window.BongoNewsAPI.loadPopularPosts(),
                window.BongoNewsAPI.loadRecentPosts()
            ]);

            // Обрабатываем результаты
            newsData = popularPosts.status === 'fulfilled' ? popularPosts.value : [];
            const recentData = recentPosts.status === 'fulfilled' ? recentPosts.value : [];
            
            renderNews(newsData, newsContainer);
            renderNews(recentData, recentContainer);
            
            // Показываем уведомление если данные пустые
            if (newsData.length === 0) {
                showNoDataWarning();
            }
            
            setupEventListeners();
            setupSettings();
            loadLikesFromStorage();
            
        } catch (error) {
            console.error('Initialization failed:', error);
            showError();
        }
    }

    async function checkAPIHealth() {
        try {
            // Пробуем сделать простой запрос к API
            await fetch('/api/posts?limit=1');
            return true;
        } catch (error) {
            console.warn('API health check failed:', error);
            return false;
        }
    }

    function showLoading() {
        newsContainer.innerHTML = `
            <div class="loading">
                <i class="fas fa-spinner fa-spin" style="font-size: 3rem; margin-bottom: 20px; color: var(--cat-primary);"></i>
                <h3>Загрузка новостей...</h3>
                <p>Подключаемся к серверу</p>
            </div>
        `;
        recentContainer.innerHTML = `
            <div class="loading">
                <i class="fas fa-spinner fa-spin"></i> Загрузка...
            </div>
        `;
    }

    function showWarning() {
        const warning = document.createElement('div');
        warning.className = 'search-info';
        warning.style.backgroundColor = '#fff3cd';
        warning.style.borderLeftColor = '#ffc107';
        warning.innerHTML = `
            <p><i class="fas fa-exclamation-triangle"></i> 
            Сервер временно недоступен. Показываем демо-данные.</p>
        `;
        
        if (newsContainer) {
            newsContainer.parentNode.insertBefore(warning, newsContainer);
        }
    }

    function showNoDataWarning() {
        newsContainer.innerHTML += `
            <div class="search-info" style="grid-column: 1 / -1;">
                <p><i class="fas fa-info-circle"></i> 
                Нет данных для отображения. Попробуйте обновить страницу или проверьте подключение к серверу.</p>
            </div>
        `;
    }

    function showError() {
        newsContainer.innerHTML = `
            <div class="loading">
                <i class="fas fa-exclamation-triangle" style="color: #ff6b6b; font-size: 3rem; margin-bottom: 20px;"></i>
                <h3>Ошибка подключения</h3>
                <p>Не удалось загрузить новости. Возможные причины:</p>
                <ul style="text-align: left; max-width: 400px; margin: 20px auto;">
                    <li>Сервер не запущен</li>
                    <li>Проблемы с сетью</li>
                    <li>База данных недоступна</li>
                </ul>
                <button class="search-btn" onclick="location.reload()" style="margin-top: 20px;">
                    <i class="fas fa-redo"></i> Попробовать снова
                </button>
                <button class="search-btn" onclick="useDemoData()" style="margin-top: 10px; background-color: #6c757d;">
                    <i class="fas fa-eye"></i> Показать демо-данные
                </button>
            </div>
        `;
        recentContainer.innerHTML = '<div class="loading">Ошибка загрузки</div>';
    }

    function useDemoData() {
        // Загружаем демо-данные из API клиента
        newsData = window.BongoNewsAPI._getFallbackData();
        renderNews(newsData, newsContainer);
        renderNews(newsData.slice(0, 4), recentContainer);
    }

    function renderNews(news, container) {
        container.innerHTML = '';
        
        if (!news || news.length === 0) {
            container.innerHTML = `
                <div class="loading" style="grid-column: 1 / -1;">
                    <i class="fas fa-newspaper" style="font-size: 2rem; color: #adb5bd;"></i>
                    <p>Нет новостей для отображения</p>
                </div>
            `;
            return;
        }

        news.forEach(item => {
            const newsCard = document.createElement('article');
            newsCard.className = 'news-card';
            newsCard.dataset.category = item.category;
            newsCard.dataset.id = item.id;

            const likes = getPostLikes(item.id);
            const liked = getLikedPosts().includes(item.id);

            newsCard.innerHTML = `
                <img src="${item.image}" alt="${item.title}" class="news-image" loading="lazy">
                <div class="news-content">
                    <h2 class="news-title">${escapeHtml(item.title)}</h2>
                    <p class="news-excerpt">${escapeHtml(item.excerpt)}</p>
                    <div class="news-tags">
                        ${item.tags.map(tag => `<span class="news-tag">${escapeHtml(tag)}</span>`).join('')}
                    </div>
                    <div class="news-meta">
                        <div>
                            <span class="news-date">${item.date}</span>
                            <span class="news-category category-${item.category}">${getCategoryName(item.category)}</span>
                        </div>
                        <div class="like-section">
                            <button class="heart-btn ${liked ? 'liked' : ''}" aria-label="Поставить лайк">
                                <span class="heart-icon">❤️</span>
                            </button>
                            <span class="like-count">${likes}</span>
                        </div>
                    </div>
                </div>
            `;
            container.appendChild(newsCard);
        });

        // Добавляем обработчики лайков
        container.querySelectorAll('.heart-btn').forEach(btn => {
            btn.addEventListener('click', function(e) {
                e.stopPropagation();
                likePost(this);
            });
        });

        // Добавляем обработчики кликов по карточкам
        container.querySelectorAll('.news-card').forEach(card => {
            card.addEventListener('click', function() {
                const postId = this.dataset.id;
                // Здесь можно добавить переход на детальную страницу поста
                console.log('Post clicked:', postId);
                
                // Добавляем в недавно просмотренные
                addToRecentlyViewed(postId, this.querySelector('.news-title').textContent);
            });
        });
    }

    function escapeHtml(text) {
        const div = document.createElement('div');
        div.textContent = text;
        return div.innerHTML;
    }

    function addToRecentlyViewed(postId, title) {
        // Сохраняем в localStorage
        const viewed = JSON.parse(localStorage.getItem('recentlyViewed') || '[]');
        const item = { id: postId, title: title, timestamp: Date.now() };
        
        // Удаляем если уже есть
        const existingIndex = viewed.findIndex(item => item.id === postId);
        if (existingIndex !== -1) {
            viewed.splice(existingIndex, 1);
        }
        
        // Добавляем в начало
        viewed.unshift(item);
        
        // Ограничиваем до 10 последних
        if (viewed.length > 10) {
            viewed.pop();
        }
        
        localStorage.setItem('recentlyViewed', JSON.stringify(viewed));
    }

    function getCategoryName(category) {
        const categories = {
            'technology': 'Технологии',
            'science': 'Наука',
            'culture': 'Культура'
        };
        return categories[category] || category;
    }

    function setupEventListeners() {
        // Фильтры по категориям
        filterButtons.forEach(button => {
            button.addEventListener('click', function() {
                const category = this.dataset.category;
                filterButtons.forEach(btn => btn.classList.remove('active'));
                this.classList.add('active');
                currentFilter = category;
                applyFilter();
            });
        });

        // Поиск по кнопке
        searchButton.addEventListener('click', performSearch);

        // Поиск по Enter
        searchInput.addEventListener('keypress', function(e) {
            if (e.key === 'Enter') {
                performSearch();
            }
        });

        // Поиск по тегам
        searchTags.forEach(tag => {
            tag.addEventListener('click', function() {
                const searchText = this.textContent;
                searchInput.value = searchText;
                performSearch();
            });
        });

        // Открытие настроек
        const settingsBtn = document.getElementById('settings-btn');
        if (settingsBtn) {
            settingsBtn.addEventListener('click', function(e) {
                e.preventDefault();
                document.getElementById('settings-modal').style.display = 'block';
            });
        }
    }

    async function performSearch() {
        const searchTerm = searchInput.value.trim();
        
        if (searchTerm === '') {
            showToast('Пожалуйста, введите поисковый запрос', 'warning');
            return;
        }

        try {
            // Показываем индикатор загрузки
            searchButton.innerHTML = '<i class="fas fa-spinner fa-spin"></i> Поиск...';
            searchButton.disabled = true;

            const searchResults = await window.BongoNewsAPI.searchPosts(searchTerm, currentFilter);
            
            // Сохраняем результаты поиска
            localStorage.setItem('searchQuery', searchTerm);
            localStorage.setItem('searchResults', JSON.stringify(searchResults));
            
            // Переходим на страницу результатов
            window.location.href = 'search-results.html';
        } catch (error) {
            console.error('Search failed:', error);
            showToast('Ошибка при выполнении поиска. Попробуйте еще раз.', 'error');
        } finally {
            // Восстанавливаем кнопку
            searchButton.innerHTML = '<i class="fas fa-search"></i> Найти';
            searchButton.disabled = false;
        }
    }

    function showToast(message, type = 'info') {
        const toast = document.createElement('div');
        toast.className = `toast toast-${type}`;
        toast.innerHTML = `
            <span>${message}</span>
            <button onclick="this.parentElement.remove()">×</button>
        `;
        
        // Стили для toast
        toast.style.cssText = `
            position: fixed;
            top: 20px;
            right: 20px;
            padding: 12px 20px;
            background: ${type === 'error' ? '#f8d7da' : type === 'warning' ? '#fff3cd' : '#d1ecf1'};
            color: ${type === 'error' ? '#721c24' : type === 'warning' ? '#856404' : '#0c5460'};
            border: 1px solid ${type === 'error' ? '#f5c6cb' : type === 'warning' ? '#ffeaa7' : '#bee5eb'};
            border-radius: 4px;
            z-index: 10000;
            display: flex;
            align-items: center;
            justify-content: space-between;
            min-width: 300px;
            box-shadow: 0 4px 12px rgba(0,0,0,0.1);
        `;
        
        document.body.appendChild(toast);
        setTimeout(() => toast.remove(), 5000);
    }

    function applyFilter() {
        if (currentFilter === 'all') {
            renderNews(newsData, newsContainer);
        } else {
            const filteredNews = newsData.filter(item => item.category === currentFilter);
            renderNews(filteredNews, newsContainer);
        }
    }

    async function likePost(button) {
        const newsCard = button.closest('.news-card');
        const likeCount = button.nextElementSibling;
        const newsId = newsCard.dataset.id;
        
        let likedPosts = getLikedPosts();
        let currentCount = parseInt(likeCount.textContent) || 0;

        if (button.classList.contains('liked')) {
            // Убираем лайк
            button.classList.remove('liked');
            likeCount.textContent = Math.max(0, currentCount - 1);
            updateLikesInData(newsId, Math.max(0, currentCount - 1));
            likedPosts = likedPosts.filter(id => id !== newsId);
        } else {
            // Ставим лайк
            button.classList.add('liked');
            likeCount.textContent = currentCount + 1;
            updateLikesInData(newsId, currentCount + 1);
            likedPosts.push(newsId);
            animateBongoCat();
            
            // Отправляем лайк на сервер
            const success = await window.BongoNewsAPI.likePost(newsId);
            if (!success) {
                // Показываем сообщение
                showToast('Не удалось поставить лайк. Сохраняем локально.', 'warning');
            } else {
                showToast('Лайк сохранен!', 'success');
            }
        }
        
        saveLikesToStorage();
        saveLikedPosts(likedPosts);
    }

    function getLikedPosts() {
        return JSON.parse(localStorage.getItem('likedPosts') || '[]');
    }
    
    function saveLikedPosts(ids) {
        localStorage.setItem('likedPosts', JSON.stringify(ids));
    }

    function getPostLikes(postId) {
        const likesData = JSON.parse(localStorage.getItem('newsLikes') || '{}');
        return likesData[postId] || 0;
    }

    function updateLikesInData(id, likes) {
        const likesData = JSON.parse(localStorage.getItem('newsLikes') || '{}');
        likesData[id] = likes;
        localStorage.setItem('newsLikes', JSON.stringify(likesData));
    }

    function animateBongoCat() {
        const bubble = document.getElementById('heart-bubble');
        if (bubble) {
            bubble.classList.add('show');
            setTimeout(() => bubble.classList.remove('show'), 1500);
        }
    }

    function saveLikesToStorage() {
        // Данные уже сохраняются в updateLikesInData
    }

    function loadLikesFromStorage() {
        // Лайки загружаются при рендеринге через getPostLikes
    }

    // Настройки
    function setupSettings() {
        const settingsModal = document.getElementById('settings-modal');
        const closeSettings = document.getElementById('close-settings');
        const cancelSettings = document.getElementById('cancel-settings');
        const saveSettings = document.getElementById('save-settings');

        if (settingsModal) {
            // Закрытие модального окна
            const closeModal = () => settingsModal.style.display = 'none';
            
            closeSettings.addEventListener('click', closeModal);
            cancelSettings.addEventListener('click', closeModal);
            
            saveSettings.addEventListener('click', () => {
                // Сохранение настроек
                saveSettingsToLocalStorage();
                closeModal();
                showToast('Настройки сохранены!', 'success');
            });

            // Закрытие по клику вне окна
            window.addEventListener('click', (e) => {
                if (e.target === settingsModal) {
                    closeModal();
                }
            });
            
            // Загрузка сохраненных настроек
            loadSettingsFromLocalStorage();
        }
    }

    function saveSettingsToLocalStorage() {
        const settings = {
            theme: document.querySelector('input[name="theme"]:checked').value,
            textSize: document.querySelector('input[name="text-size"]:checked').value,
            emailNotifications: document.getElementById('email-notifications').checked,
            pushNotifications: document.getElementById('push-notifications').checked,
            newsletter: document.getElementById('newsletter').checked,
            language: document.getElementById('language-select').value
        };
        
        localStorage.setItem('bongoNewsSettings', JSON.stringify(settings));
        applySettings(settings);
    }

    function loadSettingsFromLocalStorage() {
        const saved = JSON.parse(localStorage.getItem('bongoNewsSettings') || '{}');
        
        // Применяем настройки
        if (saved.theme) {
            const themeRadio = document.querySelector(`input[name="theme"][value="${saved.theme}"]`);
            if (themeRadio) themeRadio.checked = true;
        }
        
        if (saved.textSize) {
            const sizeRadio = document.querySelector(`input[name="text-size"][value="${saved.textSize}"]`);
            if (sizeRadio) sizeRadio.checked = true;
        }
        
        if (saved.emailNotifications !== undefined) {
            document.getElementById('email-notifications').checked = saved.emailNotifications;
        }
        
        if (saved.pushNotifications !== undefined) {
            document.getElementById('push-notifications').checked = saved.pushNotifications;
        }
        
        if (saved.newsletter !== undefined) {
            document.getElementById('newsletter').checked = saved.newsletter;
        }
        
        if (saved.language) {
            document.getElementById('language-select').value = saved.language;
        }
        
        applySettings(saved);
    }

    function applySettings(settings) {
        // Применение темы
        if (settings.theme === 'dark') {
            document.documentElement.setAttribute('data-theme', 'dark');
        } else if (settings.theme === 'light') {
            document.documentElement.setAttribute('data-theme', 'light');
        } else {
            document.documentElement.removeAttribute('data-theme');
        }
        
        // Применение размера текста
        if (settings.textSize) {
            document.body.style.fontSize = settings.textSize === 'small' ? '14px' : 
                                           settings.textSize === 'large' ? '18px' : '16px';
        }
        
        // Применение языка
        if (settings.language) {
            document.documentElement.lang = settings.language;
        }
    }

    // Запускаем инициализацию
    await init();
});
[file content end]