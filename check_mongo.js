print("=== ПРОВЕРКА СОДЕРЖИМОГО MONGODB ===");

// 1. Коллекция posts - вероятно кэшированные/обработанные посты
if (db.posts) {
    var count = db.posts.countDocuments();
    print("posts: " + count + " документов");
    if (count > 0) {
        print("Структура документа:");
        var doc = db.posts.findOne();
        for (var key in doc) {
            if (key !== "_id") { // Пропустим ObjectId
                print("  " + key + ": " + typeof doc[key] + " (" + 
                      (typeof doc[key] === 'object' ? 'object' : doc[key]) + ")");
            }
        }
    } else {
        print("posts коллекция ПУСТАЯ!");
    }
} else {
    print("posts коллекция НЕ СУЩЕСТВУЕТ!");
}

print("\n---");

// 2. user_interactions - взаимодействия пользователей
if (db.user_interactions) {
    var count = db.user_interactions.countDocuments();
    print("user_interactions: " + count + " документов");
    if (count > 0) {
        print("Пример документа:");
        printjson(db.user_interactions.findOne());
    }
} else {
    print("user_interactions коллекция НЕ СУЩЕСТВУЕТ!");
}

print("\n---");

// 3. top_posts_view - view для аналитики
if (db.top_posts_view) {
    print("top_posts_view - это view");
    try {
        var result = db.top_posts_view.find().limit(1);
        if (result) {
            print("View содержит данные");
        }
    } catch (e) {
        print("Ошибка чтения view: " + e.message);
    }
} else {
    print("top_posts_view НЕ СУЩЕСТВУЕТ!");
}

print("\n=== ВСЕ КОЛЛЕКЦИИ ===");
var collections = db.getCollectionNames();
print("Всего коллекций: " + collections.length);
collections.forEach(function(name) {
    var stats = db[name].stats();
    print(name + ": " + stats.count + " документов, размер: " + 
          Math.round(stats.size / 1024) + " KB");
});

print("\n=== СТАТИСТИКА БАЗЫ ===");
printjson(db.stats());
