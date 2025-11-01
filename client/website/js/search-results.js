document.addEventListener('DOMContentLoaded', function() {
    //новостей (те же самые, что и на главной странице)
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

    const searchInput = document.getElementById('search-input');
    const searchButton = document.getElementById('search-btn');
    const resultsContainer = document.getElementById('search-results-container');
    const searchInfo = document.getElementById('search-info');
    const noResults = document.getElementById('no-results');
    const filterButtons = document.querySelectorAll('.filter-btn');
    let currentFilter = 'all';
    let currentSearchTerm = '';

    function init() {
        const searchQuery = localStorage.getItem('searchQuery');
        
        if (searchQuery) {
            searchInput.value = searchQuery;
            performSearch(searchQuery);
        } else {
            window.location.href = 'index.html';
        }

        setupEventListeners();
        loadLikesFromStorage();
    }

    function performSearch(searchTerm) {
        currentSearchTerm = searchTerm.toLowerCase();
        const searchResults = newsData.filter(item => {
            const inTags = item.tags.some(tag => 
                tag.toLowerCase().includes(currentSearchTerm)
            );
            const inTitle = item.title.toLowerCase().includes(currentSearchTerm);
            const inExcerpt = item.excerpt.toLowerCase().includes(currentSearchTerm);
            
            return inTags || inTitle || inExcerpt;
        });

        displaySearchResults(searchResults, searchTerm);
    }

    function displaySearchResults(results, searchTerm) {
        searchInfo.innerHTML = `
            <p>По запросу "<strong>${searchTerm}</strong>" найдено <strong>${results.length}</strong> результатов</p>
        `;

        if (results.length === 0) {
            resultsContainer.style.display = 'none';
            noResults.style.display = 'block';
        } else {
            resultsContainer.style.display = 'grid';
            noResults.style.display = 'none';
            renderNews(results);
        }
    }

    function renderNews(news) {
        resultsContainer.innerHTML = '';
        
        news.forEach(item => {
            const newsCard = document.createElement('article');
            newsCard.className = 'news-card';
            newsCard.dataset.category = item.category;
            newsCard.dataset.id = item.id;

            const likes = item.likes || 0;
            const liked = getLikedPosts().includes(item.id);

            const highlightedTags = item.tags.map(tag => {
                if (tag.toLowerCase().includes(currentSearchTerm)) {
                    return `<span class="news-tag highlighted">${tag}</span>`;
                }
                return `<span class="news-tag">${tag}</span>`;
            }).join('');

            newsCard.innerHTML = `
                <img src="${item.image}" alt="${item.title}" class="news-image">
                <div class="news-content">
                    <h2 class="news-title">${highlightTitle(item.title, currentSearchTerm)}</h2>
                    <p class="news-excerpt">${highlightText(item.excerpt, currentSearchTerm)}</p>
                    <div class="news-tags">
                        ${highlightedTags}
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
            resultsContainer.appendChild(newsCard);
        });

        document.querySelectorAll('.heart-btn').forEach(btn => {
            btn.addEventListener('click', function() {
                likePost(this);
            });
        });
    }

    function highlightTitle(title, searchTerm) {
        if (!searchTerm) return title;
        const regex = new RegExp(`(${searchTerm})`, 'gi');
        return title.replace(regex, '<mark>$1</mark>');
    }

    function highlightText(text, searchTerm) {
        if (!searchTerm) return text;
        const regex = new RegExp(`(${searchTerm})`, 'gi');
        return text.replace(regex, '<mark>$1</mark>');
    }

    function setupEventListeners() {
        searchButton.addEventListener('click', () => {
            const searchTerm = searchInput.value.trim();
            if (searchTerm) {
                localStorage.setItem('searchQuery', searchTerm);
                performSearch(searchTerm);
            }
        });

        searchInput.addEventListener('keypress', function(e) {
            if (e.key === 'Enter') {
                const searchTerm = searchInput.value.trim();
                if (searchTerm) {
                    localStorage.setItem('searchQuery', searchTerm);
                    performSearch(searchTerm);
                }
            }
        });

        document.querySelectorAll('.search-tag').forEach(tag => {
            tag.addEventListener('click', function() {
                const searchText = this.textContent;
                searchInput.value = searchText;
                localStorage.setItem('searchQuery', searchText);
                performSearch(searchText);
            });
        });

        filterButtons.forEach(button => {
            button.addEventListener('click', function() {
                const category = this.dataset.category;
                filterButtons.forEach(btn => btn.classList.remove('active'));
                this.classList.add('active');
                currentFilter = category;
                applyFilter();
            });
        });
    }

    function applyFilter() {
        const searchQuery = localStorage.getItem('searchQuery');
        if (!searchQuery) return;

        let filteredResults = newsData.filter(item => {
            const matchesSearch = item.tags.some(tag => 
                tag.toLowerCase().includes(searchQuery.toLowerCase())
            ) || item.title.toLowerCase().includes(searchQuery.toLowerCase()) ||
              item.excerpt.toLowerCase().includes(searchQuery.toLowerCase());

            if (currentFilter === 'all') {
                return matchesSearch;
            } else {
                return matchesSearch && item.category === currentFilter;
            }
        });

        displaySearchResults(filteredResults, searchQuery);
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
        }
    }

    function getCategoryName(category) {
        const categories = {
            'technology': 'Технологии',
            'science': 'Наука',
            'culture': 'Культура'
        };
        return categories[category] || category;
    }

    init();
});