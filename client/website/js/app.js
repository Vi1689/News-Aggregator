document.addEventListener('DOMContentLoaded', function() {
    //тут данные новостей
    const newsData = [
        {
            id: 1,
            title: "Технологии будущего: что нас ждет в 2024 году",
            excerpt: "Ученые прогнозируют прорыв в области искусственного интеллекта и квантовых вычислений в наступающем году...",
            image: "https://picsum.photos/600/400?random=1",
            date: "15 ноября 2023",
            category: "technology",
            tags: ["технологии", "искусственный интеллект", "будущее", "инновации"],
            likes: 0
        },
        {
            id: 2,
            title: "Климатические изменения: новые данные",
            excerpt: "Международная группа исследователей опубликовала отчет о темпах глобального потепления за последнее десятилетие...",
            image: "https://picsum.photos/600/400?random=2",
            date: "14 ноября 2023",
            category: "science",
            tags: ["климат", "наука", "экология", "глобальное потепление"],
            likes: 0
        },
        {
            id: 3,
            title: "Искусство в цифровую эпоху: новые тенденции",
            excerpt: "Цифровые художники и NFT продолжают менять представление о современном искусстве и его ценности...",
            image: "https://picsum.photos/600/400?random=3",
            date: "13 ноября 2023",
            category: "culture",
            tags: ["искусство", "цифровое искусство", "nft", "культура"],
            likes: 0
        },
        {
            id: 4,
            title: "Новый прорыв в квантовых компьютерах",
            excerpt: "Компания IBM анонсировала новый квантовый процессор с рекордным количеством кубитов...",
            image: "https://picsum.photos/600/400?random=4",
            date: "12 ноября 2023",
            category: "technology",
            tags: ["квантовые компьютеры", "ibm", "технологии", "инновации"],
            likes: 0
        },
        {
            id: 5,
            title: "Открытие новой экзопланеты в зоне обитаемости",
            excerpt: "Астрономы обнаружили планету, которая может поддерживать условия для существования жизни...",
            image: "https://picsum.photos/600/400?random=5",
            date: "11 ноября 2023",
            category: "science",
            tags: ["астрономия", "экзопланеты", "космос", "наука"],
            likes: 0
        },
        {
            id: 6,
            title: "Фестиваль современного искусства в Венеции",
            excerpt: "Венецианская биеннале представляет новые работы художников со всего мира...",
            image: "https://picsum.photos/600/400?random=6",
            date: "10 ноября 2023",
            category: "culture",
            tags: ["фестиваль", "венеция", "современное искусство", "культура"],
            likes: 0
        },
        {
            id: 7,
            title: "Искусственный интеллект в медицине: революционный подход",
            excerpt: "Новые алгоритмы ИИ помогают диагностировать заболевания с невероятной точностью...",
            image: "https://picsum.photos/600/400?random=7",
            date: "9 ноября 2023",
            category: "technology",
            tags: ["искусственный интеллект", "медицина", "здравоохранение", "технологии"],
            likes: 0
        },
        {
            id: 8,
            title: "Исследование океанских глубин: новые открытия",
            excerpt: "Ученые обнаружили неизвестные виды морских организмов в Марианской впадине...",
            image: "https://picsum.photos/600/400?random=8",
            date: "8 ноября 2023",
            category: "science",
            tags: ["океан", "исследования", "биология", "наука"],
            likes: 0
        }
    ];

    const newsContainer = document.getElementById('news-container');
    const filterButtons = document.querySelectorAll('.filter-btn');
    const searchInput = document.querySelector('.search-input');
    const searchButton = document.querySelector('.search-btn');
    const searchTags = document.querySelectorAll('.search-tag');
    let currentFilter = 'all';


function init() {
    renderNews(newsData);
    setupEventListeners();
    setupSettings();
    loadLikesFromStorage();
}

    function renderNews(news) {
        newsContainer.innerHTML = '';
        if (news.length === 0) {
            newsContainer.innerHTML = '<div class="loading">Новости не найдены</div>';
            return;
        }
        news.forEach(item => {
            const newsCard = document.createElement('article');
            newsCard.className = 'news-card';
            newsCard.dataset.category = item.category;
            newsCard.dataset.id = item.id;

            const likes = item.likes || 0;
            const liked = getLikedPosts().includes(item.id);

            newsCard.innerHTML = `
                <img src="${item.image}" alt="${item.title}" class="news-image">
                <div class="news-content">
                    <h2 class="news-title">${item.title}</h2>
                    <p class="news-excerpt">${item.excerpt}</p>
                    <div class="news-tags">
                        ${item.tags.map(tag => `<span class="news-tag">${tag}</span>`).join('')}
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
            newsContainer.appendChild(newsCard);
        });

        document.querySelectorAll('.heart-btn').forEach(btn => {
            btn.addEventListener('click', function() {
                likePost(this);
            });
            btn.addEventListener('keypress', function(e) {
                if (e.key === 'Enter') likePost(this);
            });
        });
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

        // поиск по кнопке
        searchButton.addEventListener('click', performSearch);

        // поиск по Enter
        searchInput.addEventListener('keypress', function(e) {
            if (e.key === 'Enter') {
                performSearch();
            }
        });

        //  поиск по тегам
        searchTags.forEach(tag => {
            tag.addEventListener('click', function() {
                const searchText = this.textContent;
                searchInput.value = searchText;
                performSearch();
            });
        });
    }

    function performSearch() {
        const searchTerm = searchInput.value.trim().toLowerCase();
        
        if (searchTerm === '') {
            alert('Пожалуйста, введите поисковый запрос');
            return;
        }

        //поисковый запрос в localStorage для использования на странице результатов
        localStorage.setItem('searchQuery', searchTerm);
        
        // реходим на страницу результатов
        window.location.href = 'search-results.html';
    }

    function applyFilter() {
        if (currentFilter === 'all') {
            renderNews(newsData);
        } else {
            const filteredNews = newsData.filter(item => item.category === currentFilter);
            renderNews(filteredNews);
        }
    }

    function likePost(button) {
        const newsCard = button.closest('.news-card');
        const likeCount = button.nextElementSibling;
        const newsId = parseInt(newsCard.dataset.id);
        let currentCount = parseInt(likeCount.textContent);

        let likedPosts = getLikedPosts();
        if (button.classList.contains('liked')) {
            button.classList.remove('liked');
            likeCount.textContent = currentCount = Math.max(0, currentCount - 1);
            updateLikesInData(newsId, currentCount);
            likedPosts = likedPosts.filter(id => id !== newsId);
        } else {
            button.classList.add('liked');
            likeCount.textContent = currentCount = currentCount + 1;
            updateLikesInData(newsId, currentCount);
            likedPosts.push(newsId);
            animateBongoCat();
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

    function updateLikesInData(id, likes) {
        const newsItem = newsData.find(item => item.id === id);
        if (newsItem) newsItem.likes = likes;
    }

    function animateBongoCat() {
        const bubble = document.getElementById('heart-bubble');
        if (bubble) {
            bubble.classList.add('show');
            setTimeout(() => bubble.classList.remove('show'), 1500);
        }
    }

    function saveLikesToStorage() {
        const likesData = {};
        newsData.forEach(item => { likesData[item.id] = item.likes; });
        localStorage.setItem('newsLikes', JSON.stringify(likesData));
    }

    function loadLikesFromStorage() {
        const likesData = JSON.parse(localStorage.getItem('newsLikes'));
        if (likesData) {
            newsData.forEach(item => {
                if (likesData[item.id] !== undefined)
                    item.likes = likesData[item.id];
            });
            applyFilter();
        }
    }

    init();
});