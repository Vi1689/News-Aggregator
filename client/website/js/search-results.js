[file name]: client/website/js/search-results.js
[file content begin]
document.addEventListener('DOMContentLoaded', async function() {
    const searchInput = document.getElementById('search-input');
    const searchButton = document.getElementById('search-btn');
    const resultsContainer = document.getElementById('search-results-container');
    const searchInfo = document.getElementById('search-info');
    const noResults = document.getElementById('no-results');
    const filterButtons = document.querySelectorAll('.filter-btn');
    let currentFilter = 'all';
    let searchResults = [];

    async function init() {
        const searchQuery = localStorage.getItem('searchQuery');
        
        if (searchQuery) {
            searchInput.value = searchQuery;
            await performSearch(searchQuery);
        } else {
            window.location.href = 'index.html';
        }

        setupEventListeners();
    }

    async function performSearch(searchTerm) {
        try {
            // Показываем индикатор загрузки
            searchButton.innerHTML = '<i class="fas fa-spinner fa-spin"></i> Поиск...';
            searchButton.disabled = true;
            resultsContainer.innerHTML = '<div class="loading"><i class="fas fa-spinner fa-spin"></i> Поиск...</div>';

            searchResults = await window.BongoNewsAPI.searchPosts(searchTerm, currentFilter);
            displaySearchResults(searchResults, searchTerm);
            
        } catch (error) {
            console.error('Search failed:', error);
            displaySearchResults([], searchTerm);
        } finally {
            searchButton.innerHTML = '<i class="fas fa-search"></i> Найти';
            searchButton.disabled = false;
        }
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

            const likes = getPostLikes(item.id);
            const liked = getLikedPosts().includes(item.id);
            const searchTerm = searchInput.value.toLowerCase();

            const highlightedTags = item.tags.map(tag => {
                if (tag.toLowerCase().includes(searchTerm)) {
                    return `<span class="news-tag highlighted">${tag}</span>`;
                }
                return `<span class="news-tag">${tag}</span>`;
            }).join('');

            newsCard.innerHTML = `
                <img src="${item.image}" alt="${item.title}" class="news-image" loading="lazy">
                <div class="news-content">
                    <h2 class="news-title">${highlightTitle(item.title, searchTerm)}</h2>
                    <p class="news-excerpt">${highlightText(item.excerpt, searchTerm)}</p>
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

        // Обработчики лайков
        resultsContainer.querySelectorAll('.heart-btn').forEach(btn => {
            btn.addEventListener('click', function(e) {
                e.stopPropagation();
                likePost(this);
            });
        });

        // Обработчики кликов по карточкам
        resultsContainer.querySelectorAll('.news-card').forEach(card => {
            card.addEventListener('click', function() {
                const postId = this.dataset.id;
                console.log('Post clicked:', postId);
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

        if (currentFilter === 'all') {
            renderNews(searchResults);
        } else {
            const filteredResults = searchResults.filter(item => item.category === currentFilter);
            renderNews(filteredResults);
        }
    }

    async function likePost(button) {
        const newsCard = button.closest('.news-card');
        const likeCount = button.nextElementSibling;
        const newsId = parseInt(newsCard.dataset.id);
        
        let likedPosts = getLikedPosts();
        let currentCount = parseInt(likeCount.textContent);

        if (button.classList.contains('liked')) {
            button.classList.remove('liked');
            likeCount.textContent = Math.max(0, currentCount - 1);
            updateLikesInData(newsId, currentCount - 1);
            likedPosts = likedPosts.filter(id => id !== newsId);
        } else {
            button.classList.add('liked');
            likeCount.textContent = currentCount + 1;
            updateLikesInData(newsId, currentCount + 1);
            likedPosts.push(newsId);
            animateBongoCat();
            
            const success = await window.BongoNewsAPI.likePost(newsId);
            if (!success) {
                button.classList.remove('liked');
                likeCount.textContent = currentCount;
                updateLikesInData(newsId, currentCount);
                likedPosts = likedPosts.filter(id => id !== newsId);
            }
        }
        
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

    function getCategoryName(category) {
        const categories = {
            'technology': 'Технологии',
            'science': 'Наука',
            'culture': 'Культура'
        };
        return categories[category] || category;
    }

    await init();
});
[file content end]