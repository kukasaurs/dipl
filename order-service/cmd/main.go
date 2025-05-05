package main

import (
	"cleaning-app/order-service/internal/config"
	"cleaning-app/order-service/internal/handler"
	"cleaning-app/order-service/internal/repository"
	"cleaning-app/order-service/internal/services"
	"cleaning-app/order-service/internal/utils"
	"context"
	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"log"
	"net/http"
)

func main() {
	// 1. Создание базового контекста и менеджера завершения
	baseCtx := context.Background()
	ctx, shutdownManager := utils.NewShutdownManager(baseCtx)
	shutdownManager.StartListening()

	// 2. Загрузка конфигурации
	cfg, err := config.LoadConfig()
	if err != nil {
		log.Fatal("Не удалось загрузить конфигурацию:", err)
	}

	// 3. Инициализация MongoDB
	mongoClient, err := mongo.Connect(ctx, options.Client().ApplyURI(cfg.MongoURI))
	if err != nil {
		log.Fatal("Не удалось подключиться к MongoDB:", err)
	}
	db := mongoClient.Database("cleaning_service")

	// Регистрация MongoDB для graceful shutdown
	shutdownManager.Register(func(ctx context.Context) error {
		log.Println("[SHUTDOWN] Закрытие соединения с MongoDB...")
		return mongoClient.Disconnect(ctx)
	})

	// 4. Инициализация Redis
	opts, err := redis.ParseURL(cfg.RedisURL)
	if err != nil {
		log.Fatal("Неверный URL Redis:", err)
	}
	rdb := redis.NewClient(opts)
	if err := rdb.Ping(ctx).Err(); err != nil {
		log.Fatal("Не удалось выполнить ping Redis:", err)
	}

	// Регистрация Redis для graceful shutdown
	shutdownManager.Register(func(ctx context.Context) error {
		log.Println("[SHUTDOWN] Закрытие соединения с Redis...")
		return rdb.Close()
	})

	// 5. Инициализация сервисов
	orderRepo := repository.NewOrderRepository(db)
	notifier := services.NewNotificationService()
	orderService := services.NewOrderService(orderRepo, notifier, rdb)
	orderHandler := handler.NewOrderHandler(orderService)

	// 6. Запуск фонового обновления кэша
	cacheRefresher := services.NewCacheRefresher(orderService, rdb)
	cacheRefresher.Start(ctx)

	// 7. Настройка маршрутов
	router := gin.Default()
	router.Use(utils.AuthMiddleware(cfg.AuthServiceURL))

	orders := router.Group("/api/orders")
	{
		orders.POST("/", orderHandler.CreateOrder)
		orders.GET("/my", orderHandler.GetMyOrders)
		orders.PUT("/:id/update", orderHandler.UpdateOrder)
		orders.DELETE("/:id/delete", orderHandler.DeleteOrder)

		protected := orders.Group("/")
		protected.Use(utils.RequireRoles("manager", "admin"))
		{
			protected.PUT("/:id/assign", orderHandler.AssignCleaner)
			protected.PUT("/:id/unassign", orderHandler.UnassignCleaner)
			protected.GET("/all", orderHandler.GetAllOrders)
			protected.GET("/filter", orderHandler.FilterOrders)
		}

		protectedCleaner := orders.Group("/")
		protectedCleaner.Use(utils.RequireRoles("cleaner"))
		{
			protectedCleaner.PUT("/:id/confirm", orderHandler.ConfirmCompletion)
			protectedCleaner.GET("/cleaner/my", orderHandler.GetMyAssignedOrders)
			protectedCleaner.GET("/:id", orderHandler.GetOrderDetails)
			protectedCleaner.PUT("/:id/reject", orderHandler.RejectAssignedOrder)
		}
	}

	// 8. Запуск HTTP-сервера
	server := &http.Server{
		Addr:    cfg.ServerPort,
		Handler: router,
	}

	go func() {
		log.Println("Сервис заказов запущен на порту :8001")
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Ошибка сервера: %v", err)
		}
	}()

	// Регистрация graceful shutdown для сервера
	shutdownManager.Register(func(ctx context.Context) error {
		log.Println("[SHUTDOWN] Остановка HTTP-сервера...")
		return server.Shutdown(ctx)
	})

	// Ожидание завершения
	select {}
}
