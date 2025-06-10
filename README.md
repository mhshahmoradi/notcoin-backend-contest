# NotCoin Backend Contest

A high-performance flash sale system built with Go, PostgreSQL, and Redis. Handles concurrent purchases with user limits and checkout code management.

## 🚀 Features

- **Hourly Flash Sales**: 10,000 items per sale cycle
- **Concurrent Purchase Handling**: Race condition protection
- **User Limits**: Max 10 items per user per sale
- **Checkout Codes**: Temporary reservation system with TTL
- **Redis Caching**: Fast checkout code validation
- **Database Transactions**: ACID compliance for purchases

## 🏗️ Architecture

```
┌─────────────────┬─────────────────┬─────────────────┐
│   HTTP Layer    │  Service Layer  │  Storage Layer  │
├─────────────────┼─────────────────┼─────────────────┤
│ CheckoutHandler │                 │                 │
│ PurchaseHandler │  SaleService    │ PostgreSQL      │
│                 │                 │ Redis           │
└─────────────────┴─────────────────┴─────────────────┘
```

## 🛠️ Setup & Installation

### Prerequisites
- Docker & Docker Compose

### Run Application
```bash
docker-compose up -d
```

Application runs on port **8032**

## 📡 API Endpoints

### 1. Checkout (Reserve Item)
```bash
curl -X POST "http://localhost:8032/checkout?user_id=user123&id=1001"
```

**Response:**
```json
{
  "code": "a1b2c3d4e5f6g7h8"
}
```

### 2. Purchase (Complete Transaction)
```bash
curl -X POST "http://localhost:8032/purchase?code=a1b2c3d4e5f6g7h8"
```

**Success Response:**
```json
{
  "status": "success",
  "message": "Item purchased successfully",
  "item_id": 1001
}
```

**Error Response:**
```json
{
  "status": "failed",
  "message": "User has reached the purchase limit for this sale"
}
```

## ⚡ Performance Testing

Run comprehensive performance tests:

```bash
./run_all_tests_sequential.sh
```

This will execute load tests with various concurrency levels and scenarios.

## 🔧 Implementation Details

### Core Components

**1. Sale Management**
- Hourly sale cycles with automatic deactivation
- 10,000 items generated per sale
- Database transactions ensure consistency

**2. Checkout Process**
- Validates active sale and item availability
- Checks user purchase limits (max 10 per sale)
- Generates unique checkout codes with TTL
- Stores codes in both Redis and PostgreSQL

**3. Purchase Process**
- Validates checkout codes from Redis (primary) or DB (fallback)
- Executes atomic transaction with row-level locking
- Updates item status, sale counters, and user limits
- Prevents race conditions and overselling

**4. Error Handling**
- Custom error types for different failure scenarios
- Proper HTTP status codes
- Detailed error messages for debugging

### Concurrency & Performance

**Race Condition Prevention:**
- Database row-level locking with `FOR UPDATE`
- Atomic transactions for critical operations
- Redis caching for fast validation

**Scalability Features:**
- Connection pooling for database
- Redis for high-speed caching
- Efficient batch operations
- Structured logging for monitoring

## 📁 Project Structure

```
├── cmd/
│   ├── server/main.go      # HTTP server entry point
│   └── migrate/main.go     # Database migrations
├── internal/
│   ├── config/            # Configuration management
│   ├── handler/           # HTTP request handlers
│   ├── models/            # Data structures
│   ├── service/           # Business logic
│   └── store/             # Data access layer
├── migrations/            # SQL migration files
└── README.md
```

## ⚡ Performance Metrics

**Load Test Results (1000 concurrent users - Scenario: Buy All Items):**
- **Total Items Sold**: 10,000 items in **15.4 seconds**
- **Purchase Rate**: 649.75 purchases/second
- **Success Rate**: 100% (0 errors)
- **HTTP Request Rate**: 1,299.5 requests/second
- **Average Response Time**: 734ms

*Test environment: MacBook M1 Air with 8GB RAM*

## 🐛 Troubleshooting

**Common Issues:**
- Ensure Docker and Docker Compose are installed
- Check containers are running: `docker-compose ps`
- View logs: `docker-compose logs`
- Restart services: `docker-compose restart`

---

Built with ❤️ for the NotCoin Backend Contest
